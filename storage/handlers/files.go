package handlers

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/tags"
	"github.com/spf13/viper"
	"net/http"
	"net/url"
	"nkonev.name/storage/auth"
	"nkonev.name/storage/client"
	"nkonev.name/storage/dto"
	. "nkonev.name/storage/logger"
	"nkonev.name/storage/utils"
	"sort"
	"strconv"
	"strings"
	"time"
)

type FilesHandler struct {
	minio       *minio.Client
	chatClient  *client.RestClient
	minioConfig *utils.MinioConfig
}

type RenameDto struct {
	Newname string `json:"newname"`
}

const filesMultipartKey = "files"
const UrlStorageGetFile = "/storage/public/download"
const UrlStorageGetFilePublicExternal = "/public/download"

type FileInfoDto struct {
	Id           string    `json:"id"`
	Filename     string    `json:"filename"`
	Url          string    `json:"url"`
	PublicUrl    *string   `json:"publicUrl"`
	Size         int64     `json:"size"`
	CanRemove    bool      `json:"canRemove"`
	CanShare     bool      `json:"canShare"`
	LastModified time.Time `json:"lastModified"`
	OwnerId      int64     `json:"ownerId"`
	Owner        *dto.User `json:"owner"`
}

const publicKey = "public"

func NewFilesHandler(
	minio *minio.Client,
	chatClient *client.RestClient,
	minioConfig *utils.MinioConfig,
) *FilesHandler {
	return &FilesHandler{
		minio:       minio,
		chatClient:  chatClient,
		minioConfig: minioConfig,
	}
}

func serializeTags(public bool) map[string]string {
	var userTags = map[string]string{}
	userTags[publicKey] = fmt.Sprintf("%v", public)
	return userTags
}

func deserializeTags(tagging *tags.Tags) (bool, error) {
	if tagging == nil {
		return false, nil
	}

	var tagsMap map[string]string = tagging.ToMap()
	publicString, ok := tagsMap[publicKey]
	if !ok {
		return false, nil
	}
	return utils.ParseBoolean(publicString)
}

type uploadDto struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

func (h *FilesHandler) UploadHandler(c echo.Context) error {
	var userPrincipalDto, ok = c.Get(utils.USER_PRINCIPAL_DTO).(*auth.AuthResult)
	if !ok {
		GetLogEntry(c.Request()).Errorf("Error during getting auth context")
		return errors.New("Error during getting auth context")
	}
	chatId, err := utils.ParseInt64(c.Param("chatId"))
	if err != nil {
		return err
	}
	if ok, err := h.chatClient.CheckAccess(userPrincipalDto.UserId, chatId); err != nil {
		return c.NoContent(http.StatusInternalServerError)
	} else if !ok {
		return c.NoContent(http.StatusUnauthorized)
	}

	bucketName := h.minioConfig.Files

	fileItemUuid := uuid.New().String()

	fileItemUuidString := c.Param("fileItemUuid")
	if fileItemUuidString != "" {
		fileItemUuid = fileItemUuidString
	}

	// check this fileItem belongs to user
	filenameChatPrefix := fmt.Sprintf("chat/%v/%v/", chatId, fileItemUuid)
	belongs, err := h.checkFileItemBelongsToUser(filenameChatPrefix, c, chatId, bucketName, userPrincipalDto)
	if err != nil {
		return err
	}
	if !belongs {
		return c.NoContent(http.StatusUnauthorized)
	}
	// end check

	//form, err := c.MultipartForm()
	//if err != nil {
	//	return err
	//}
	//files := form.File[filesMultipartKey]

	var fDto = uploadDto{}
	if err := c.Bind(&fDto); err != nil {
		Logger.Errorf("Unable to read body %v", err)
		return err
	}

	var pu string
	//for _, file := range files {
	// TODO Think how to really (if possible) check file size.
	//  Seems answer here https://stackoverflow.com/questions/17313695/how-to-restrict-the-size-of-the-file-being-uploaded-on-to-the-aws-s3-service-usi?rq=1
	userLimitOk, _, _, err := checkUserLimit(h.minio, bucketName, userPrincipalDto, fDto.Size)
	if err != nil {
		return err
	}
	if !userLimitOk {
		return c.JSON(http.StatusRequestEntityTooLarge, &utils.H{"status": "fail"})
	}

	//contentType := file.Header.Get("Content-Type")
	dotExt := getDotExtensionStr(fDto.Filename)

	//Logger.Debugf("Determined content type: %v", contentType)

	//src, err := file.Open()
	//if err != nil {
	//	return err
	//}
	//defer src.Close()

	fileUuid := uuid.New().String()
	filename := fmt.Sprintf("chat/%v/%v/%v%v", chatId, fileItemUuid, fileUuid, dotExt)

	var userMetadata = serializeMetadataByArgs(fDto.Filename, userPrincipalDto, chatId)

	//if _, err := h.minio.PutObject(context.Background(), bucketName, filename, src, file.Size, minio.PutObjectOptions{ContentType: contentType, UserMetadata: userMetadata}); err != nil {
	//	Logger.Errorf("Error during upload object: %v", err)
	//	return err
	//}
	//h.minio.Pu
	const xAmzMetaPrefix = "X-Amz-Meta-"
	uv := url.Values{}
	for k, v := range userMetadata {
		uv[strings.Title(xAmzMetaPrefix+k)] = []string{v}
	}
	puU, _ := h.minio.Presign(context.Background(), http.MethodPut, bucketName, filename, viper.GetDuration("minio.files.presignDuration"), uv)
	pu = puU.String()
	//}

	// get count
	count := h.getCountFilesInFileItem(bucketName, filenameChatPrefix)

	Logger.Infof("==========> %v", pu)
	return c.JSON(http.StatusOK, &utils.H{"status": "ok", "fileItemUuid": fileItemUuid, "count": count, "presignedUrl": pu})
}

type ReplaceTextFileDto struct {
	Id          string `json:"id"` // file id
	Text        string `json:"text"`
	ContentType string `json:"contentType"`
	Filename    string `json:"filename"`
}

func (h *FilesHandler) ReplaceHandler(c echo.Context) error {
	var userPrincipalDto, ok = c.Get(utils.USER_PRINCIPAL_DTO).(*auth.AuthResult)
	if !ok {
		GetLogEntry(c.Request()).Errorf("Error during getting auth context")
		return errors.New("Error during getting auth context")
	}
	chatId, err := utils.ParseInt64(c.Param("chatId"))
	if err != nil {
		return err
	}
	if ok, err := h.chatClient.CheckAccess(userPrincipalDto.UserId, chatId); err != nil {
		return c.NoContent(http.StatusInternalServerError)
	} else if !ok {
		return c.NoContent(http.StatusUnauthorized)
	}

	var bindTo = new(ReplaceTextFileDto)
	if err := c.Bind(bindTo); err != nil {
		GetLogEntry(c.Request()).Warnf("Error during binding to dto %v", err)
		return err
	}

	bucketName := h.minioConfig.Files

	fileItemUuid := getFileItemUuid(bindTo.Id)

	// check this fileItem belongs to user
	filenameChatPrefix := fmt.Sprintf("chat/%v/%v/", chatId, fileItemUuid)
	belongs, err := h.checkFileItemBelongsToUser(filenameChatPrefix, c, chatId, bucketName, userPrincipalDto)
	if err != nil {
		return err
	}
	if !belongs {
		return c.NoContent(http.StatusUnauthorized)
	}
	// end check

	fileSize := int64(len(bindTo.Text))
	userLimitOk, _, _, err := checkUserLimit(h.minio, bucketName, userPrincipalDto, fileSize)
	if err != nil {
		return err
	}
	if !userLimitOk {
		return c.JSON(http.StatusRequestEntityTooLarge, &utils.H{"status": "fail"})
	}

	contentType := bindTo.ContentType
	dotExt := getDotExtensionStr(bindTo.Filename)

	Logger.Debugf("Determined content type: %v", contentType)

	src := strings.NewReader(bindTo.Text)

	fileUuid := getFileId(bindTo.Id)
	filename := fmt.Sprintf("chat/%v/%v/%v%v", chatId, fileItemUuid, fileUuid, dotExt)

	var userMetadata = serializeMetadataByArgs(bindTo.Filename, userPrincipalDto, chatId)

	if _, err := h.minio.PutObject(context.Background(), bucketName, filename, src, fileSize, minio.PutObjectOptions{ContentType: contentType, UserMetadata: userMetadata}); err != nil {
		Logger.Errorf("Error during upload object: %v", err)
		return err
	}

	return c.NoContent(http.StatusOK)
}

func (h *FilesHandler) getCountFilesInFileItem(bucketName string, filenameChatPrefix string) int {
	var count = 0
	var objectsNew <-chan minio.ObjectInfo = h.minio.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{
		Prefix:    filenameChatPrefix,
		Recursive: true,
	})
	count = len(objectsNew)
	for oi := range objectsNew {
		Logger.Debugf("Processing %v", oi.Key)
		count++
	}
	return count
}

func getFileItemUuid(fileId string) string {
	split := strings.Split(fileId, "/")
	return split[2]
}

func getFileId(fileId string) string {
	split := strings.Split(fileId, "/")
	filenameWithExt := split[3]
	splitFn := strings.Split(filenameWithExt, ".")
	return splitFn[0]
}

func (h *FilesHandler) ListHandler(c echo.Context) error {
	var userPrincipalDto, ok = c.Get(utils.USER_PRINCIPAL_DTO).(*auth.AuthResult)
	if !ok {
		GetLogEntry(c.Request()).Errorf("Error during getting auth context")
		return errors.New("Error during getting auth context")
	}
	chatId, err := utils.ParseInt64(c.Param("chatId"))
	if err != nil {
		return err
	}
	if ok, err := h.chatClient.CheckAccess(userPrincipalDto.UserId, chatId); err != nil {
		return c.NoContent(http.StatusInternalServerError)
	} else if !ok {
		return c.NoContent(http.StatusUnauthorized)
	}

	fileItemUuid := c.QueryParam("fileItemUuid")

	bucketName := h.minioConfig.Files

	Logger.Debugf("Listing bucket '%v':", bucketName)

	var filenameChatPrefix string
	if fileItemUuid == "" {
		filenameChatPrefix = fmt.Sprintf("chat/%v/", chatId)
	} else {
		filenameChatPrefix = fmt.Sprintf("chat/%v/%v/", chatId, fileItemUuid)
	}

	list, err := h.getListFilesInFileItem(userPrincipalDto.UserId, bucketName, filenameChatPrefix, chatId)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, &utils.H{"status": "ok", "files": list})
}

func getUsersRemotelyOrEmpty(userIdSet map[int64]bool, restClient *client.RestClient) map[int64]*dto.User {
	if remoteUsers, err := getUsersRemotely(userIdSet, restClient); err != nil {
		Logger.Warn("Error during getting users from aaa")
		return map[int64]*dto.User{}
	} else {
		return remoteUsers
	}
}

func getUsersRemotely(userIdSet map[int64]bool, restClient *client.RestClient) (map[int64]*dto.User, error) {
	var userIds = utils.SetToArray(userIdSet)
	length := len(userIds)
	Logger.Infof("Requested user length is %v", length)
	if length == 0 {
		return map[int64]*dto.User{}, nil
	}
	users, err := restClient.GetUsers(userIds)
	if err != nil {
		return nil, err
	}
	var ownersObjects = map[int64]*dto.User{}
	for _, u := range users {
		ownersObjects[u.Id] = u
	}
	return ownersObjects, nil
}

func (h *FilesHandler) getListFilesInFileItem(behalfUserId int64, bucket, filenameChatPrefix string, chatId int64) ([]*FileInfoDto, error) {
	var objects <-chan minio.ObjectInfo = h.minio.ListObjects(context.Background(), bucket, minio.ListObjectsOptions{
		WithMetadata: true,
		Prefix:       filenameChatPrefix,
		Recursive:    true,
	})

	var list []*FileInfoDto = make([]*FileInfoDto, 0)
	for objInfo := range objects {
		Logger.Debugf("Object '%v'", objInfo.Key)
		tagging, err := h.minio.GetObjectTagging(context.Background(), bucket, objInfo.Key, minio.GetObjectTaggingOptions{})
		if err != nil {
			Logger.Errorf("Error during getting tags %v", err)
			return nil, err
		}

		info, err := h.getFileInfo(behalfUserId, objInfo, chatId, tagging, true)
		if err != nil {
			Logger.Errorf("Error get file info: %v, skipping", err)
			continue
		}

		list = append(list, info)
	}
	sort.SliceStable(list, func(i, j int) bool {
		return list[i].LastModified.Unix() < list[j].LastModified.Unix()
	})

	var participantIdSet = map[int64]bool{}
	for _, fileDto := range list {
		participantIdSet[fileDto.OwnerId] = true
	}
	var users = getUsersRemotelyOrEmpty(participantIdSet, h.chatClient)
	for _, fileDto := range list {
		user := users[fileDto.OwnerId]
		if user != nil {
			fileDto.Owner = user
		}
	}

	return list, nil
}

func (h *FilesHandler) getFileInfo(behalfUserId int64, objInfo minio.ObjectInfo, chatId int64, tagging *tags.Tags, hasAmzPrefix bool) (*FileInfoDto, error) {
	downloadUrl, err := h.getChatPrivateUrlFromObject(objInfo, chatId)
	if err != nil {
		Logger.Errorf("Error get private url: %v", err)
		return nil, err
	}
	metadata := objInfo.UserMetadata

	_, fileOwnerId, fileName, err := deserializeMetadata(metadata, hasAmzPrefix)
	if err != nil {
		Logger.Errorf("Error get metadata: %v", err)
		return nil, err
	}

	public, err := deserializeTags(tagging)
	if err != nil {
		Logger.Errorf("Error get tags: %v", err)
		return nil, err
	}

	publicUrl, err := h.getPublicUrl(public, objInfo.Key)
	if err != nil {
		Logger.Errorf("Error get public url: %v", err)
		return nil, err
	}

	info := &FileInfoDto{
		Id:           objInfo.Key,
		Filename:     fileName,
		Url:          *downloadUrl,
		Size:         objInfo.Size,
		CanRemove:    fileOwnerId == behalfUserId,
		CanShare:     fileOwnerId == behalfUserId,
		LastModified: objInfo.LastModified,
		OwnerId:      fileOwnerId,
		PublicUrl:    publicUrl,
	}
	return info, nil
}

func (h *FilesHandler) getPublicUrl(public bool, fileName string) (*string, error) {
	if !public {
		return nil, nil
	}

	downloadUrl, err := url.Parse(h.getBaseUrlForDownload() + UrlStorageGetFilePublicExternal)
	if err != nil {
		return nil, err
	}

	query := downloadUrl.Query()
	query.Add("file", fileName)
	downloadUrl.RawQuery = query.Encode()
	str := downloadUrl.String()
	return &str, nil
}

func (h *FilesHandler) getBaseUrlForDownload() string {
	return viper.GetString("server.contextPath") + "/storage"
}

func (h *FilesHandler) getChatPrivateUrlFromObject(objInfo minio.ObjectInfo, chatId int64) (*string, error) {
	downloadUrl, err := url.Parse(h.getBaseUrlForDownload() + "/download")
	if err != nil {
		return nil, err
	}

	query := downloadUrl.Query()
	query.Add("file", objInfo.Key)
	downloadUrl.RawQuery = query.Encode()
	str := downloadUrl.String()
	return &str, nil
}

type DeleteObjectDto struct {
	Id string `json:"id"` // file id
}

func (h *FilesHandler) DeleteHandler(c echo.Context) error {
	var bindTo = new(DeleteObjectDto)
	if err := c.Bind(bindTo); err != nil {
		GetLogEntry(c.Request()).Warnf("Error during binding to dto %v", err)
		return err
	}

	var userPrincipalDto, ok = c.Get(utils.USER_PRINCIPAL_DTO).(*auth.AuthResult)
	if !ok {
		GetLogEntry(c.Request()).Errorf("Error during getting auth context")
		return errors.New("Error during getting auth context")
	}
	chatId, err := utils.ParseInt64(c.Param("chatId"))
	if err != nil {
		return err
	}
	if ok, err := h.chatClient.CheckAccess(userPrincipalDto.UserId, chatId); err != nil {
		return c.NoContent(http.StatusInternalServerError)
	} else if !ok {
		return c.NoContent(http.StatusUnauthorized)
	}

	bucketName := h.minioConfig.Files

	// check this fileItem belongs to user
	objectInfo, err := h.minio.StatObject(context.Background(), bucketName, bindTo.Id, minio.StatObjectOptions{})
	if err != nil {
		Logger.Errorf("Error during getting object %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	belongs, err := h.checkFileBelongsToUser(objectInfo, chatId, userPrincipalDto, false)
	if err != nil {
		Logger.Errorf("Error during checking belong object %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if !belongs {
		Logger.Errorf("Object '%v' is not belongs to user %v", objectInfo.Key, userPrincipalDto.UserId)
		return c.NoContent(http.StatusUnauthorized)
	}
	// end check

	formerFileItemUuid := getFileItemUuid(objectInfo.Key)

	err = h.minio.RemoveObject(context.Background(), bucketName, objectInfo.Key, minio.RemoveObjectOptions{})
	if err != nil {
		Logger.Errorf("Error during removing object %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// this fileItemUuid used for display list in response
	fileItemUuid := c.QueryParam("fileItemUuid")
	var filenameChatPrefix string
	if fileItemUuid == "" {
		filenameChatPrefix = fmt.Sprintf("chat/%v/", chatId)
	} else {
		filenameChatPrefix = fmt.Sprintf("chat/%v/%v/", chatId, fileItemUuid)
	}

	list, err := h.getListFilesInFileItem(userPrincipalDto.UserId, bucketName, filenameChatPrefix, chatId)
	if err != nil {
		return err
	}

	// this fileItemUuid used for remove orphans
	if h.countFilesUnderFileUuid(chatId, formerFileItemUuid, bucketName) == 0 {
		h.chatClient.RemoveFileItem(chatId, formerFileItemUuid, userPrincipalDto.UserId)
	}

	return c.JSON(http.StatusOK, &utils.H{"status": "ok", "files": list})
}

func (h *FilesHandler) checkFileItemBelongsToUser(filenameChatPrefix string, c echo.Context, chatId int64, bucketName string, userPrincipalDto *auth.AuthResult) (bool, error) {
	var objects <-chan minio.ObjectInfo = h.minio.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{
		WithMetadata: true,
		Prefix:       filenameChatPrefix,
		Recursive:    true,
	})
	for objInfo := range objects {
		b, err := h.checkFileBelongsToUser(objInfo, chatId, userPrincipalDto, true)
		if err != nil {
			return false, err
		}
		if !b {
			return false, nil
		}
	}
	return true, nil
}

func (h *FilesHandler) checkFileBelongsToUser(objInfo minio.ObjectInfo, chatId int64, userPrincipalDto *auth.AuthResult, hasAmzPrefix bool) (bool, error) {
	gotChatId, gotOwnerId, _, err := deserializeMetadata(objInfo.UserMetadata, hasAmzPrefix)
	if err != nil {
		Logger.Errorf("Error deserializeMetadata: %v", err)
		return false, err
	}

	if gotChatId != chatId {
		Logger.Infof("Wrong chatId: expected %v but got %v", chatId, gotChatId)
		return false, nil
	}

	if gotOwnerId != userPrincipalDto.UserId {
		Logger.Infof("Wrong ownerId: expected %v but got %v", userPrincipalDto.UserId, gotOwnerId)
		return false, nil
	}
	return true, nil
}

func (h *FilesHandler) DownloadHandler(c echo.Context) error {
	var userPrincipalDto, ok = c.Get(utils.USER_PRINCIPAL_DTO).(*auth.AuthResult)
	if !ok {
		GetLogEntry(c.Request()).Errorf("Error during getting auth context")
		return errors.New("Error during getting auth context")
	}

	bucketName := h.minioConfig.Files

	// check user belongs to chat
	fileId := c.QueryParam("file")
	objectInfo, err := h.minio.StatObject(context.Background(), bucketName, fileId, minio.StatObjectOptions{})
	if err != nil {
		Logger.Errorf("Error during getting object %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	chatId, _, fileName, err := deserializeMetadata(objectInfo.UserMetadata, false)
	if err != nil {
		Logger.Errorf("Error during deserializing object metadata %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	belongs, err := h.chatClient.CheckAccess(userPrincipalDto.UserId, chatId)
	if err != nil {
		Logger.Errorf("Error during checking user auth to chat %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if !belongs {
		Logger.Errorf("User %v is not belongs to chat %v", userPrincipalDto.UserId, chatId)
		return c.NoContent(http.StatusUnauthorized)
	}
	// end check

	c.Response().Header().Set(echo.HeaderContentLength, strconv.FormatInt(objectInfo.Size, 10))
	c.Response().Header().Set(echo.HeaderContentType, objectInfo.ContentType)
	c.Response().Header().Set(echo.HeaderContentDisposition, "attachment; Filename=\""+fileName+"\"")

	object, e := h.minio.GetObject(context.Background(), bucketName, fileId, minio.GetObjectOptions{})
	if e != nil {
		return c.JSON(http.StatusInternalServerError, &utils.H{"status": "fail"})
	}
	defer object.Close()

	return c.Stream(http.StatusOK, objectInfo.ContentType, object)
}

type PublishRequest struct {
	Public bool   `json:"public"`
	Id     string `json:"id"`
}

func (h *FilesHandler) SetPublic(c echo.Context) error {
	var userPrincipalDto, ok = c.Get(utils.USER_PRINCIPAL_DTO).(*auth.AuthResult)
	if !ok {
		GetLogEntry(c.Request()).Errorf("Error during getting auth context")
		return errors.New("Error during getting auth context")
	}

	bucketName := h.minioConfig.Files

	var bindTo = new(PublishRequest)
	if err := c.Bind(bindTo); err != nil {
		GetLogEntry(c.Request()).Warnf("Error during binding to dto %v", err)
		return err
	}

	// check user is owner
	fileId := bindTo.Id
	objectInfo, err := h.minio.StatObject(context.Background(), bucketName, fileId, minio.StatObjectOptions{})
	if err != nil {
		Logger.Errorf("Error during getting object %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	chatId, ownerId, _, err := deserializeMetadata(objectInfo.UserMetadata, false)
	if err != nil {
		Logger.Errorf("Error during deserializing object metadata %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if ownerId != userPrincipalDto.UserId {
		Logger.Errorf("User %v is not owner of file %v", userPrincipalDto.UserId, fileId)
		return c.NoContent(http.StatusUnauthorized)
	}
	// end check

	tagsMap := serializeTags(bindTo.Public)
	objectTags, err := tags.MapToObjectTags(tagsMap)
	if err != nil {
		Logger.Errorf("Error during mapping tags %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	err = h.minio.PutObjectTagging(context.Background(), bucketName, fileId, objectTags, minio.PutObjectTaggingOptions{})
	if err != nil {
		Logger.Errorf("Error during saving tags %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	objectInfo, err = h.minio.StatObject(context.Background(), bucketName, fileId, minio.StatObjectOptions{})
	if err != nil {
		Logger.Errorf("Error during stat %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	tagging, err := h.minio.GetObjectTagging(context.Background(), bucketName, fileId, minio.GetObjectTaggingOptions{})
	if err != nil {
		Logger.Errorf("Error during getting tags %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	info, err := h.getFileInfo(userPrincipalDto.UserId, objectInfo, chatId, tagging, false)
	if err != nil {
		Logger.Errorf("Error during getFileInfo %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	var participantIdSet = map[int64]bool{}
	participantIdSet[userPrincipalDto.UserId] = true
	var users = getUsersRemotelyOrEmpty(participantIdSet, h.chatClient)
	user, ok := users[userPrincipalDto.UserId]
	if ok {
		info.Owner = user
	}

	return c.JSON(http.StatusOK, info)
}

type CountResponse struct {
	Count int `json:"count"`
}

func (h *FilesHandler) CountHandler(c echo.Context) error {
	var userPrincipalDto, ok = c.Get(utils.USER_PRINCIPAL_DTO).(*auth.AuthResult)
	if !ok {
		GetLogEntry(c.Request()).Errorf("Error during getting auth context")
		return errors.New("Error during getting auth context")
	}

	bucketName := h.minioConfig.Files

	// check user belongs to chat
	fileItemUuid := c.Param("fileItemUuid")
	chatIdString := c.Param("chatId")
	chatId, err := utils.ParseInt64(chatIdString)
	if err != nil {
		Logger.Errorf("Error during parsing chatId %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	belongs, err := h.chatClient.CheckAccess(userPrincipalDto.UserId, chatId)
	if err != nil {
		Logger.Errorf("Error during checking user auth to chat %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if !belongs {
		Logger.Errorf("User %v is not belongs to chat %v", userPrincipalDto.UserId, chatId)
		return c.NoContent(http.StatusUnauthorized)
	}
	// end check

	counter := h.countFilesUnderFileUuid(chatId, fileItemUuid, bucketName)

	var countDto = CountResponse{
		Count: counter,
	}

	return c.JSON(http.StatusOK, countDto)
}

func (h *FilesHandler) countFilesUnderFileUuid(chatId int64, fileItemUuid string, bucketName string) int {
	var filenameChatPrefix = fmt.Sprintf("chat/%v/%v/", chatId, fileItemUuid)
	var objects <-chan minio.ObjectInfo = h.minio.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{
		WithMetadata: false,
		Prefix:       filenameChatPrefix,
		Recursive:    true,
	})

	var counter = 0
	for _ = range objects {
		counter++
	}
	return counter
}

func (h *FilesHandler) PublicDownloadHandler(c echo.Context) error {
	bucketName := h.minioConfig.Files

	// check file is public
	fileId := c.QueryParam("file")
	objectInfo, err := h.minio.StatObject(context.Background(), bucketName, fileId, minio.StatObjectOptions{})
	if err != nil {
		Logger.Errorf("Error during getting object %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	_, _, fileName, err := deserializeMetadata(objectInfo.UserMetadata, false)
	if err != nil {
		Logger.Errorf("Error during deserializing object metadata %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	tagging, err := h.minio.GetObjectTagging(context.Background(), bucketName, fileId, minio.GetObjectTaggingOptions{})
	if err != nil {
		Logger.Errorf("Error during deserializing object tags %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	isPublic, err := deserializeTags(tagging)
	if err != nil {
		Logger.Errorf("Error during deserializing object tags %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if !isPublic {
		Logger.Errorf("File %v is not public", fileId)
		return c.NoContent(http.StatusUnauthorized)
	}
	// end check

	c.Response().Header().Set(echo.HeaderContentLength, strconv.FormatInt(objectInfo.Size, 10))
	c.Response().Header().Set(echo.HeaderContentType, objectInfo.ContentType)
	c.Response().Header().Set(echo.HeaderContentDisposition, "attachment; Filename=\""+fileName+"\"")

	object, e := h.minio.GetObject(context.Background(), bucketName, fileId, minio.GetObjectOptions{})
	if e != nil {
		return c.JSON(http.StatusInternalServerError, &utils.H{"status": "fail"})
	}
	defer object.Close()

	return c.Stream(http.StatusOK, objectInfo.ContentType, object)
}

func (h *FilesHandler) LimitsHandler(c echo.Context) error {
	var userPrincipalDto, ok = c.Get(utils.USER_PRINCIPAL_DTO).(*auth.AuthResult)
	if !ok {
		GetLogEntry(c.Request()).Errorf("Error during getting auth context")
		return errors.New("Error during getting auth context")
	}
	chatId, err := utils.ParseInt64(c.Param("chatId"))
	if err != nil {
		return err
	}
	if ok, err := h.chatClient.CheckAccess(userPrincipalDto.UserId, chatId); err != nil {
		return c.NoContent(http.StatusInternalServerError)
	} else if !ok {
		return c.NoContent(http.StatusUnauthorized)
	}

	bucketName := h.minioConfig.Files

	desiredSize, err := utils.ParseInt64(c.QueryParam("desiredSize"))
	if err != nil {
		return err
	}
	ok, consumption, available, err := checkUserLimit(h.minio, bucketName, userPrincipalDto, desiredSize)
	if err != nil {
		return err
	}

	if !ok {
		return c.JSON(http.StatusOK, &utils.H{"status": "oversized", "used": consumption, "available": available})
	} else {
		return c.JSON(http.StatusOK, &utils.H{"status": "ok", "used": consumption, "available": available})
	}
}
