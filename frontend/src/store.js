import Vue from 'vue'
import Vuex from 'vuex'
import axios from "axios";

Vue.use(Vuex);

export const GET_USER = 'getUser';
export const SET_USER = 'setUser';
export const UNSET_USER = 'unsetUser';
export const FETCH_USER_PROFILE = 'fetchUserProfile';
export const FETCH_AVAILABLE_OAUTH2_PROVIDERS = 'fetchAvailableOauth2';
export const GET_AVAILABLE_OAUTH2_PROVIDERS = 'getAvailableOauth2';
export const SET_AVAILABLE_OAUTH2_PROVIDERS = 'setAvailableOauth2';
export const GET_SEARCH_STRING = 'getSearchString';
export const SET_SEARCH_STRING = 'setSearchString';
export const UNSET_SEARCH_STRING = 'unsetSearchString';

export const GET_TITLE = 'getTitle';
export const SET_TITLE = 'setTitle';
export const GET_SHOW_SEARCH = 'getShowSearch';
export const SET_SHOW_SEARCH = 'setShowSearch';
export const GET_CHAT_ID = 'getChatId';
export const SET_CHAT_ID = 'setChatId';
export const GET_CHAT_USERS_COUNT = 'getChatUsesCount';
export const SET_CHAT_USERS_COUNT = 'setChatUsesCount';
export const GET_VIDEO_CHAT_USERS_COUNT = 'getVideoChatUsesCount';
export const SET_VIDEO_CHAT_USERS_COUNT = 'setVideoChatUsesCount';
export const GET_SHOW_CALL_BUTTON = 'getShowCallButton';
export const SET_SHOW_CALL_BUTTON = 'setShowCallButton';
export const GET_SHOW_HANG_BUTTON = 'getShowHangButton';
export const SET_SHOW_HANG_BUTTON = 'setShowHangButton';
export const GET_SHOW_RECORD_START_BUTTON = 'getShowRecordStartButton';
export const SET_SHOW_RECORD_START_BUTTON = 'setShowRecordStartButton';
export const GET_SHOW_RECORD_STOP_BUTTON = 'getShowRecordStopButton';
export const SET_SHOW_RECORD_STOP_BUTTON = 'setShowRecordStopButton';
export const GET_CAN_MAKE_RECORD = 'getCanMakeRecord';
export const SET_CAN_MAKE_RECORD = 'setCanMakeRecord';
export const GET_SHOW_CHAT_EDIT_BUTTON = 'getChatEditButton';
export const SET_SHOW_CHAT_EDIT_BUTTON = 'setChatEditButton';
export const GET_CAN_BROADCAST_TEXT_MESSAGE = 'setCanBroadcastText';
export const SET_CAN_BROADCAST_TEXT_MESSAGE = 'getCanBroadcastText';
export const GET_SHOW_ALERT = 'getShowAlert';
export const SET_SHOW_ALERT = 'setShowAlert';
export const GET_LAST_ERROR = 'getLastError';
export const SET_LAST_ERROR = 'setLastError';
export const GET_ERROR_COLOR = 'getErrorColor';
export const SET_ERROR_COLOR = 'setErrorColor';

const store = new Vuex.Store({
    state: {
        currentUser: null,
        searchString: null,
        muteVideo: false,
        muteAudio: false,
        title: "",
        isShowSearch: true,
        chatId: null,
        invitedChatId: null,
        chatUsersCount: 0,
        videoChatUsersCount: 0,
        showCallButton: false,
        showHangButton: false,
        showRecordStartButton: false,
        showRecordStopButton: false,
        canMakeRecord: false,
        shareScreen: false,
        showChatEditButton: false,
        availableOAuth2Providers: [],
        canBroadcastTextMessage: false,
        showAlert: false,
        lastError: "",
        errorColor: "",
    },
    mutations: {
        [SET_USER](state, payload) {
            state.currentUser = payload;
        },
        [SET_SEARCH_STRING](state, payload) {
            state.searchString = payload;
        },
        [UNSET_USER](state) {
            state.currentUser = null;
        },
        [UNSET_SEARCH_STRING](state) {
            state.searchString = "";
        },
        [SET_SHOW_CALL_BUTTON](state, payload) {
            state.showCallButton = payload;
        },
        [SET_SHOW_HANG_BUTTON](state, payload) {
            state.showHangButton = payload;
        },
        [SET_SHOW_RECORD_START_BUTTON](state, payload) {
            state.showRecordStartButton = payload;
        },
        [SET_SHOW_RECORD_STOP_BUTTON](state, payload) {
            state.showRecordStopButton = payload;
        },
        [SET_CAN_MAKE_RECORD](state, payload) {
            state.canMakeRecord = payload;
        },
        [SET_VIDEO_CHAT_USERS_COUNT](state, payload) {
            state.videoChatUsersCount = payload;
        },
        [SET_TITLE](state, payload) {
            state.title = payload;
        },
        [SET_SHOW_SEARCH](state, payload) {
            state.isShowSearch = payload;
        },
        [SET_CHAT_USERS_COUNT](state, payload) {
            state.chatUsersCount = payload;
        },
        [SET_SHOW_CHAT_EDIT_BUTTON](state, payload) {
            state.showChatEditButton = payload;
        },
        [SET_CHAT_ID](state, payload) {
            state.chatId = payload;
        },
        [SET_AVAILABLE_OAUTH2_PROVIDERS](state, payload) {
            state.availableOAuth2Providers = payload;
        },
        [SET_CAN_BROADCAST_TEXT_MESSAGE](state, payload) {
            state.canBroadcastTextMessage = payload;
        },
        [SET_SHOW_ALERT](state, payload) {
            state.showAlert = payload;
        },
        [SET_LAST_ERROR](state, payload) {
            state.lastError = payload;
        },
        [SET_ERROR_COLOR](state, payload) {
            state.errorColor = payload;
        },
    },
    getters: {
        [GET_USER](state) {
            return state.currentUser;
        },
        [GET_SEARCH_STRING](state) {
            return state.searchString;
        },
        [GET_SHOW_CALL_BUTTON](state) {
            return state.showCallButton;
        },
        [GET_SHOW_HANG_BUTTON](state) {
            return state.showHangButton;
        },
        [GET_SHOW_RECORD_START_BUTTON](state) {
            return state.showRecordStartButton;
        },
        [GET_SHOW_RECORD_STOP_BUTTON](state) {
            return state.showRecordStopButton;
        },
        [GET_CAN_MAKE_RECORD](state) {
            return state.canMakeRecord;
        },
        [GET_VIDEO_CHAT_USERS_COUNT](state) {
            return state.videoChatUsersCount;
        },
        [GET_TITLE](state) {
            return state.title;
        },
        [GET_SHOW_SEARCH](state) {
            return state.isShowSearch;
        },
        [GET_CHAT_USERS_COUNT](state) {
            return state.chatUsersCount;
        },
        [GET_SHOW_CHAT_EDIT_BUTTON](state) {
            return state.showChatEditButton;
        },
        [GET_CHAT_ID](state) {
            return state.chatId;
        },
        [GET_AVAILABLE_OAUTH2_PROVIDERS](state) {
            return state.availableOAuth2Providers;
        },
        [GET_CAN_BROADCAST_TEXT_MESSAGE](state) {
            return state.canBroadcastTextMessage;
        },
        [GET_SHOW_ALERT](state) {
            return state.showAlert;
        },
        [GET_LAST_ERROR](state) {
            return state.lastError;
        },
        [GET_ERROR_COLOR](state) {
            return state.errorColor;
        },
    },
    actions: {
        [FETCH_USER_PROFILE](context) {
            axios.get(`/api/profile`).then(( {data} ) => {
                console.debug("fetched profile =", data);
                context.commit(SET_USER, data);
            });
        },
        [FETCH_AVAILABLE_OAUTH2_PROVIDERS](context) {
            axios.get(`/api/oauth2/providers`).then(( {data} ) => {
                console.debug("fetched oauth2 providers =", data);
                context.commit(SET_AVAILABLE_OAUTH2_PROVIDERS, data);
            });
        },
    }
});

export default store;