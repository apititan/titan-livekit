/*package com.github.nkonev.aaa.config;

import com.github.nkonev.aaa.Constants;
import org.springframework.core.annotation.Order;
import org.springframework.stereotype.Component;
import javax.servlet.*;
import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpServletRequestWrapper;
import javax.servlet.http.Part;
import java.io.IOException;
import java.util.*;

class RequestWithoutParts extends HttpServletRequestWrapper {

    public RequestWithoutParts(HttpServletRequest request) {
        super(request);
    }
    @Override
    public Collection<Part> getParts() throws IOException, ServletException {
        return new ArrayList<>();
    }
}

@Component
@Order(-2147483000)
public class AuthorizationNormalizerFilter implements Filter {

    @Override
    public void doFilter(ServletRequest request, ServletResponse response, FilterChain chain) throws IOException, ServletException {
        HttpServletRequest httpRequest = (HttpServletRequest) request;
        if ((Constants.Urls.INTERNAL_API + Constants.Urls.PROFILE).equals(httpRequest.getRequestURI())) {
            httpRequest = new RequestWithoutParts(httpRequest);
        }
        chain.doFilter(httpRequest, response);
    }
}
*/