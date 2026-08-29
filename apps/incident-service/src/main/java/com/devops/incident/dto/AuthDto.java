package com.devops.incident.dto;

import lombok.*;

public class AuthDto {

    @Getter
    @Setter
    @NoArgsConstructor
    @AllArgsConstructor
    @Builder
    public static class LoginRequest {
        private String username;
        private String password;
        private String role; // "SRE_LEAD", "DEVOPS_ENGINEER", "EVALUATOR"
    }

    @Getter
    @Setter
    @NoArgsConstructor
    @AllArgsConstructor
    @Builder
    public static class AuthResponse {
        private boolean authenticated;
        private String token;
        private String userId;
        private String username;
        private String email;
        private String role;
        private String avatarUrl;
        private String message;
    }
}
