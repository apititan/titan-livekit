package com.github.nkonev.aaa;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.gargoylesoftware.htmlunit.WebClient;
import com.github.nkonev.aaa.config.webdriver.SeleniumProperties;
import com.github.nkonev.aaa.it.OAuth2EmulatorTests;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.web.client.TestRestTemplate;

public class AbstractSeleniumRunner extends OAuth2EmulatorTests {

    private Logger LOGGER = LoggerFactory.getLogger(AbstractSeleniumRunner.class);

    protected WebClient webClient;

    @Autowired
    private SeleniumProperties seleniumProperties;

    @BeforeEach
    public void beforeSelenium() {
        LOGGER.debug("Executing before");
        webClient = new WebClient();
        webClient.getOptions().setCssEnabled(true);
        webClient.getOptions().setJavaScriptEnabled(true);
        webClient.getOptions().setScreenHeight(seleniumProperties.getWindowHeight());
        webClient.getOptions().setScreenWidth(seleniumProperties.getWindowWidth());
        webClient.getOptions().setTimeout(seleniumProperties.getImplicitlyWaitTimeout());
        webClient.getOptions().setRedirectEnabled(true);
    }

    @AfterEach
    public void afterSelenium() {
        LOGGER.debug("Executing after");
        webClient.close();
    }

    @Autowired
    protected TestRestTemplate testRestTemplate;

    @Autowired
    protected ObjectMapper objectMapper;


}
