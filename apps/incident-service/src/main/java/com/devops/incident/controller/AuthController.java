package com.devops.incident.controller;

import com.devops.incident.dto.AuthDto;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.UUID;

@RestController
@RequestMapping("/api/v1/auth")
@CrossOrigin(origins = "*")
public class AuthController {

    @PostMapping("/login")
    public ResponseEntity<AuthDto.AuthResponse> login(@RequestBody AuthDto.LoginRequest request) {
        String username = (request.getUsername() != null && !request.getUsername().isBlank())
                ? request.getUsername().trim()
                : "RamanRed";
        String role = (request.getRole() != null && !request.getRole().isBlank())
                ? request.getRole()
                : "SRE_LEAD";

        String token = "sre-token-" + UUID.randomUUID().toString().substring(0, 12);

        return ResponseEntity.ok(AuthDto.AuthResponse.builder()
                .authenticated(true)
                .token(token)
                .userId(username.toLowerCase().replaceAll("\\s+", "-"))
                .username(username)
                .email(username.toLowerCase() + "@devops.sre.io")
                .role(role)
                .avatarUrl("https://github.com/" + username + ".png")
                .message("Authentication successful")
                .build());
    }

    @GetMapping("/me")
    public ResponseEntity<AuthDto.AuthResponse> getCurrentUser(@RequestParam(defaultValue = "ramanred") String userId) {
        return ResponseEntity.ok(AuthDto.AuthResponse.builder()
                .authenticated(true)
                .token("sre-session-active")
                .userId(userId)
                .username("RamanRed")
                .email("ramanred@devops.sre.io")
                .role("SRE_LEAD")
                .avatarUrl("https://github.com/RamanRed.png")
                .message("Session active")
                .build());
    }
}
