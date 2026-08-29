package com.devops.incident.controller;

import com.devops.incident.dto.IntegrationDto;
import com.devops.incident.service.IntegrationService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/api/v1/integrations")
@CrossOrigin(origins = "*")
public class IntegrationController {

    private final IntegrationService integrationService;

    @Autowired
    public IntegrationController(IntegrationService integrationService) {
        this.integrationService = integrationService;
    }

    @GetMapping("/config")
    public ResponseEntity<IntegrationDto.ConfigResponse> getConfig(@RequestParam(defaultValue = "ramanred") String userId) {
        return ResponseEntity.ok(integrationService.getConfig(userId));
    }

    @PostMapping("/config")
    public ResponseEntity<IntegrationDto.ConfigResponse> saveConfig(@RequestBody IntegrationDto.ConfigRequest request) {
        if (request.getUserId() == null || request.getUserId().isBlank()) {
            request.setUserId("ramanred");
        }
        return ResponseEntity.ok(integrationService.saveConfig(request));
    }

    @PostMapping("/sync")
    public ResponseEntity<IntegrationDto.SyncResult> syncPlatforms(@RequestParam(defaultValue = "ramanred") String userId) {
        return ResponseEntity.ok(integrationService.syncPlatforms(userId));
    }
}
