package name.nkonev.users.controller;

import name.nkonev.users.Constants;
import name.nkonev.users.dto.LockDTO;
import name.nkonev.users.entity.jdbc.UserAccount;
import name.nkonev.users.repository.jdbc.UserAccountRepository;
import name.nkonev.users.service.FutureUserDetailsService;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.autoconfigure.session.SessionProperties;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Sort;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.web.bind.annotation.*;

import javax.servlet.http.HttpSession;
import javax.validation.Valid;
import java.util.List;
import java.util.Map;
import java.util.function.Function;
import java.util.stream.Collectors;

/**
 * Created by nik on 08.06.17.
 */
@RequestMapping(Constants.Urls.API)
@RestController
@Transactional
public class UserProfileController {

    @Autowired
    private UserAccountRepository userAccountRepository;

    @Autowired
    private PasswordEncoder passwordEncoder;

    @Autowired
    private FutureUserDetailsService blogUserDetailsService;

    @Autowired
    private UserAccountConverter userAccountConverter;

    @Autowired
    private UserDeleteService userDeleteService;

    @Autowired
    private SessionProperties sessionProperties;

    private static final Logger LOGGER = LoggerFactory.getLogger(UserProfileController.class);

    /**
     *
     * @param userAccount
     * @return current logged in profile
     */
    @PreAuthorize("isAuthenticated()")
    @GetMapping(value = Constants.Urls.PROFILE)
    public UserAccountDTO checkAuthenticated(@AuthenticationPrincipal UserAccountDetailsDTO userAccount, HttpSession session) {
        Long expiresAt = null;
        if (session!=null && sessionProperties.getTimeout()!=null) {
            expiresAt = session.getCreationTime() + sessionProperties.getTimeout().toMillis() ;
        }
        return UserAccountConverter.getUserSelfProfile(userAccount, null, expiresAt);
    }

    @GetMapping(value = Constants.Urls.USER)
    public Wrapper<UserAccountDTO> getUsers(
            @AuthenticationPrincipal UserAccountDetailsDTO userAccount,
            @RequestParam(value = "page", required=false, defaultValue = "0") int page,
            @RequestParam(value = "size", required=false, defaultValue = "0") int size,
            @RequestParam(value = "searchString", required=false, defaultValue = "") String searchString
    ) {
        PageRequest springDataPage = PageRequest.of(PageUtils.fixPage(page), PageUtils.fixSize(size), Sort.Direction.ASC, "id");
        searchString = searchString.trim();

        final String forDbSearch = "%" + searchString + "%";
        List<UserAccount> resultPage = userAccountRepository.findByUsernameContainsIgnoreCase(springDataPage.getPageSize(), springDataPage.getOffset(), forDbSearch);
        long resultPageCount = userAccountRepository.findByUsernameContainsIgnoreCaseCount(springDataPage.getPageSize(), springDataPage.getOffset(), forDbSearch);

        return new Wrapper<UserAccountDTO>(
                resultPage.stream().map(getConvertToUserAccountDTO(userAccount)).collect(Collectors.toList()),
                resultPageCount
        );
    }

    private Function<UserAccount, UserAccountDTO> getConvertToUserAccountDTO(UserAccountDetailsDTO currentUser) {
        return userAccount -> userAccountConverter.convertToUserAccountDTOExtended(currentUser, userAccount);
    }

    @GetMapping(value = Constants.Urls.USER + Constants.Urls.USER_ID)
    public UserAccountDTO getUser(
            @PathVariable(Constants.PathVariables.USER_ID) Long userId,
            @AuthenticationPrincipal UserAccountDetailsDTO userAccount
        ) {
        UserAccount userAccountEntity = userAccountRepository.findById(userId).orElseThrow(() -> new RuntimeException("user with id="+ userId + " not found"));
        if (userAccount!=null && userAccount.getId().equals(userAccountEntity.getId())){
            return UserAccountConverter.getUserSelfProfile(userAccount, userAccountEntity.getLastLoginDateTime(), null);
        } else {
            return userAccountConverter.convertToUserAccountDTO(userAccountEntity);
        }
    }

    @PostMapping(Constants.Urls.PROFILE)
    @PreAuthorize("isAuthenticated()")
    public EditUserDTO editProfile(
            @AuthenticationPrincipal UserAccountDetailsDTO userAccount,
            @RequestBody @Valid EditUserDTO userAccountDTO
    ) {
        if (userAccount == null) {
            throw new RuntimeException("Not authenticated user can't edit any user account. It can occurs due inpatient refactoring.");
        }

        UserAccount exists = userAccountRepository.findById(userAccount.getId()).orElseThrow(()-> new RuntimeException("Authenticated user account not found in database"));

        // check email already present
        if (exists.getEmail()!=null && !exists.getEmail().equals(userAccountDTO.getEmail()) && userAccountRepository.findByEmail(userAccountDTO.getEmail()).isPresent()) {
            LOGGER.error("editProfile: user with email '{}' already present. exiting...", exists.getEmail());
            return UserAccountConverter.convertToEditUserDto(exists); // we care for email leak...
        }
        // check login already present
        if (!exists.getUsername().equals(userAccountDTO.getLogin()) && userAccountRepository.findByUsername(userAccountDTO.getLogin()).isPresent()) {
            throw new UserAlreadyPresentException("User with login '" + userAccountDTO.getLogin() + "' is already present");
        }

        UserAccountConverter.updateUserAccountEntity(userAccountDTO, exists, passwordEncoder);
        exists = userAccountRepository.save(exists);

        blogUserDetailsService.refreshUserDetails(exists);

        return UserAccountConverter.convertToEditUserDto(exists);
    }

    @PreAuthorize("@blogSecurityService.canLock(#userAccountDetailsDTO, #lockDTO)")
    @PostMapping(Constants.Urls.USER + Constants.Urls.LOCK)
    public UserAccountDTOExtended setLocked(@AuthenticationPrincipal UserAccountDetailsDTO userAccountDetailsDTO, @RequestBody LockDTO lockDTO){
        UserAccount userAccount = blogUserDetailsService.getUserAccount(lockDTO.getUserId());
        if (lockDTO.isLock()){
            blogUserDetailsService.killSessions(lockDTO.getUserId());
        }
        userAccount.setLocked(lockDTO.isLock());
        userAccount = userAccountRepository.save(userAccount);

        return userAccountConverter.convertToUserAccountDTOExtended(userAccountDetailsDTO, userAccount);
    }

    @PreAuthorize("@blogSecurityService.canDelete(#userAccountDetailsDTO, #userId)")
    @DeleteMapping(Constants.Urls.USER)
    public long deleteUser(@AuthenticationPrincipal UserAccountDetailsDTO userAccountDetailsDTO, @RequestParam("userId") long userId){
        return userDeleteService.deleteUser(userId);
    }

    @PreAuthorize("@blogSecurityService.canChangeRole(#userAccountDetailsDTO, #userId)")
    @PostMapping(Constants.Urls.USER + Constants.Urls.ROLE)
    public UserAccountDTOExtended setRole(@AuthenticationPrincipal UserAccountDetailsDTO userAccountDetailsDTO, @RequestParam long userId, @RequestParam UserRole role){
        UserAccount userAccount = userAccountRepository.findById(userId).orElseThrow();
        userAccount.setRole(role);
        userAccount = userAccountRepository.save(userAccount);
        return userAccountConverter.convertToUserAccountDTOExtended(userAccountDetailsDTO, userAccount);
    }

    @PreAuthorize("@blogSecurityService.canSelfDelete(#userAccountDetailsDTO)")
    @DeleteMapping(Constants.Urls.PROFILE)
    public void selfDeleteUser(@AuthenticationPrincipal UserAccountDetailsDTO userAccountDetailsDTO){
        long userId = userAccountDetailsDTO.getId();
        userDeleteService.deleteUser(userId);
    }

    @PreAuthorize("isAuthenticated()")
    @DeleteMapping(Constants.Urls.PROFILE+Constants.Urls.FACEBOOK)
    public void selfDeleteBindingFacebook(@AuthenticationPrincipal UserAccountDetailsDTO userAccountDetailsDTO){
        long userId = userAccountDetailsDTO.getId();
        UserAccount userAccount = userAccountRepository.findById(userId).orElseThrow();
        userAccount.getOauthIdentifiers().setFacebookId(null);
        userAccount = userAccountRepository.save(userAccount);
        blogUserDetailsService.refreshUserDetails(userAccount);
    }

    @PreAuthorize("isAuthenticated()")
    @DeleteMapping(Constants.Urls.PROFILE+Constants.Urls.VKONTAKTE)
    public void selfDeleteBindingVkontakte(@AuthenticationPrincipal UserAccountDetailsDTO userAccountDetailsDTO){
        long userId = userAccountDetailsDTO.getId();
        UserAccount userAccount = userAccountRepository.findById(userId).orElseThrow();
        userAccount.getOauthIdentifiers().setVkontakteId(null);
        userAccount = userAccountRepository.save(userAccount);
        blogUserDetailsService.refreshUserDetails(userAccount);
    }

}