package jsonUtil

import (
    "embed"
    "log"
)

type LineFlexTemplateJsons struct {
    GoldStarIcon  []byte
    GrayStarIcon  []byte
    ReviewMessage []byte
}

//go:embed json/lineFlexTemplate/*
var embeddedFileSystem embed.FS

func LoadLineFlexTemplateJsons() LineFlexTemplateJsons {
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

    return LineFlexTemplateJsons{
        goldStarIcon,
        grayStarIcon,
        reviewMessage,
    }
}
