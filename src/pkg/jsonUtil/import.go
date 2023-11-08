package jsonUtil

import (
    "embed"
    "log"
)

type ReviewMessageLineFlexTemplateJsons struct {
    GoldStarIcon  []byte
    GrayStarIcon  []byte
    ReviewMessage []byte
}

type QuickReplySettingsLineFlexTemplateJsons struct {
    QuickReplySettings              []byte
    QuickReplySettingsMultiBusiness []byte
}

type AiReplyLineFlexTemplateJsons struct {
    AiReplyResult                []byte
    AiReplySettings              []byte
    AiReplySettingsMultiBusiness []byte
}

type AuthLineFlexTemplateJsons struct {
    AuthRequest []byte
}

type NotificationLineFlexTemplateJsons struct {
    AiReplySettingsUpdated    []byte
    QuickReplySettingsUpdated []byte
    ReviewReplied             []byte
}

//go:embed json/lineFlexTemplate/*
var embeddedFileSystem embed.FS // import files at compile time

func LoadReviewMessageLineFlexTemplateJsons() ReviewMessageLineFlexTemplateJsons {
    goldStarIcon, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/element/goldStarIcon.json")
    if err != nil {
        log.Fatal("Error reading goldStarIcon.json: ", err)
    }
    grayStarIcon, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/element/grayStarIcon.json")
    if err != nil {
        log.Fatal("Error reading grayStarIcon.json: ", err)
    }
    reviewMessage, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/review/reviewMessage.json")
    if err != nil {
        log.Fatal("Error reading reviewMessage.json: ", err)
    }

    return ReviewMessageLineFlexTemplateJsons{
        goldStarIcon,
        grayStarIcon,
        reviewMessage,
    }
}

func LoadQuickReplySettingsLineFlexTemplateJsons() QuickReplySettingsLineFlexTemplateJsons {
    quickReplySettings, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/quickReply/quickReplySettings.json")
    if err != nil {
        log.Fatal("Error reading quickReplySettings.json: ", err)
    }

    quickReplySettingsMultiBusiness, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/quickReply/quickReplySettingsMultiBusiness.json")
    if err != nil {
        log.Fatal("Error reading quickReplySettings.json: ", err)
    }

    return QuickReplySettingsLineFlexTemplateJsons{
        QuickReplySettings:              quickReplySettings,
        QuickReplySettingsMultiBusiness: quickReplySettingsMultiBusiness,
    }
}

func LoadAiReplyLineFlexTemplateJsons() AiReplyLineFlexTemplateJsons {
    aiReplyResult, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/aiReply/aiReplyResult.json")
    if err != nil {
        log.Fatal("Error reading aiReplyResult.json: ", err)
    }
    aiReplySettings, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/aiReply/aiReplySettings.json")
    if err != nil {
        log.Fatal("Error reading aiReplySettings.json: ", err)
    }
    aiReplySettingsMultiBusiness, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/aiReply/aiReplySettingsMultiBusiness.json")
    if err != nil {
        log.Fatal("Error reading aiReplySettings.json: ", err)
    }

    return AiReplyLineFlexTemplateJsons{
        aiReplyResult,
        aiReplySettings,
        aiReplySettingsMultiBusiness,
    }
}

func LoadAuthLineFlexTemplateJsons() AuthLineFlexTemplateJsons {
    authRequest, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/auth/authRequest.json")
    if err != nil {
        log.Fatal("Error reading authRequest.json: ", err)
    }

    return AuthLineFlexTemplateJsons{
        authRequest,
    }
}

func LoadNotificationLineFlexTemplateJsons() NotificationLineFlexTemplateJsons {
    reviewReplied, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/notification/reviewReplied.json")
    if err != nil {
        log.Fatal("Error reading reviewReplied.json: ", err)
    }
    aiReplySettingsUpdated, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/notification/aiReplySettingsUpdated.json")
    if err != nil {
        log.Fatal("Error reading aiReplySettingsUpdated.json: ", err)
    }
    quickReplySettingsUpdated, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/notification/quickReplySettingsUpdated.json")
    if err != nil {
        log.Fatal("Error reading quickReplySettingsUpdated.json: ", err)
    }

    return NotificationLineFlexTemplateJsons{
        aiReplySettingsUpdated,
        quickReplySettingsUpdated,
        reviewReplied,
    }
}
