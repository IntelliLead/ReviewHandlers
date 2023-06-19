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

//go:embed json/lineFlexTemplate/*
var embeddedFileSystem embed.FS

func LoadReviewMessageLineFlexTemplateJsons() ReviewMessageLineFlexTemplateJsons {
    goldStarIcon, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/goldStarIcon.json")
    if err != nil {
        log.Fatal("Error reading goldStarIcon.json: ", err)
    }
    grayStarIcon, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/grayStarIcon.json")
    if err != nil {
        log.Fatal("Error reading grayStarIcon.json: ", err)
    }
    reviewMessage, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/reviewMessage.json")
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
    quickReplySettings, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/quickReplySettings.json")
    if err != nil {
        log.Fatal("Error reading quickReplySettings.json: ", err)
    }
    quickReplySettingsNoQuickReply, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/quickReplySettings_noQuickReply.json")
    if err != nil {
        log.Fatal("Error reading quickReplySettings_noQuickReply.json: ", err)
    }

    quickReplyMessageUpdatedTextBox, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/quickReplyMessageUpdatedTextBox.json")
    if err != nil {
        log.Fatal("Error reading quickReplySettings_noQuickReply.json: ", err)
    }

    return QuickReplySettingsLineFlexTemplateJsons{
        QuickReplySettings:              quickReplySettings,
        QuickReplySettingsNoQuickReply:  quickReplySettingsNoQuickReply,
        QuickReplyMessageUpdatedTextBox: quickReplyMessageUpdatedTextBox,
    }
}
