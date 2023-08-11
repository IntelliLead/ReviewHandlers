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
    QuickReplySettingsNoQuickReply  []byte
    QuickReplyMessageUpdatedTextBox []byte
}

type AiReplyLineFlexTemplateJsons struct {
    AiReplyResult   []byte
    AiReplySettings []byte
}

type AuthLineFlexTemplateJsons struct {
    AuthRequest []byte
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
    quickReplySettingsNoQuickReply, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/quickReply/quickReplySettings_noQuickReply.json")
    if err != nil {
        log.Fatal("Error reading quickReplySettings_noQuickReply.json: ", err)
    }
    quickReplyMessageUpdatedTextBox, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/quickReply/quickReplyMessageUpdatedTextBox.json")
    if err != nil {
        log.Fatal("Error reading quickReplySettings_noQuickReply.json: ", err)
    }

    return QuickReplySettingsLineFlexTemplateJsons{
        QuickReplySettings:              quickReplySettings,
        QuickReplySettingsNoQuickReply:  quickReplySettingsNoQuickReply,
        QuickReplyMessageUpdatedTextBox: quickReplyMessageUpdatedTextBox,
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

    return AiReplyLineFlexTemplateJsons{
        aiReplyResult,
        aiReplySettings,
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
