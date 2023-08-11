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
    QuickReplySettings []byte
}

type AiReplyResultLineFlexTemplateJsons struct {
    AiReplyResult []byte
}

type AiReplySettingsLineFlexTemplateJsons struct {
    AiReplySettings []byte
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

    return QuickReplySettingsLineFlexTemplateJsons{
        QuickReplySettings: quickReplySettings,
    }
}

func LoadAiReplyResultLineFlexTemplateJsons() AiReplyResultLineFlexTemplateJsons {
    aiReplyResult, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/aiReplyResult.json")
    if err != nil {
        log.Fatal("Error reading aiReplyResult.json: ", err)
    }

    return AiReplyResultLineFlexTemplateJsons{
        aiReplyResult,
    }
}

func LoadAiReplySettingsLineFlexTemplateJsons() AiReplySettingsLineFlexTemplateJsons {
    file, err := embeddedFileSystem.ReadFile("json/lineFlexTemplate/aiReplySettings.json")
    if err != nil {
        log.Fatal("Error reading aiReplySettings.json: ", err)
    }

    return AiReplySettingsLineFlexTemplateJsons{
        file,
    }
}
