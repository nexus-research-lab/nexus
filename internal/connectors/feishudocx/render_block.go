package feishudocx

import (
	"strconv"
	"strings"
)

const (
	blockTypePage       = 1
	blockTypeText       = 2
	blockTypeHeading1   = 3
	blockTypeHeading9   = 11
	blockTypeBullet     = 12
	blockTypeOrdered    = 13
	blockTypeCode       = 14
	blockTypeQuote      = 15
	blockTypeTodo       = 17
	blockTypeDivider    = 22
	blockTypeFile       = 23
	blockTypeImage      = 27
	blockTypeSheet      = 30
	blockTypeTable      = 31
	blockTypeTableCell  = 32
	blockTypeBitable    = 33
	blockTypeQuoteGroup = 34
	blockTypeBoard      = 43
)

func blockID(block Block) string {
	if block == nil {
		return ""
	}
	value, _ := block["block_id"].(string)
	return strings.TrimSpace(value)
}

func blockType(block Block) int {
	if block == nil {
		return 0
	}
	switch value := block["block_type"].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case jsonNumber:
		parsed, _ := strconv.Atoi(value.String())
		return parsed
	default:
		return 0
	}
}

type jsonNumber interface {
	String() string
}

func blockChildren(block Block) []string {
	if block == nil {
		return nil
	}
	return stringSlice(block["children"])
}

func textFieldForBlockType(blockType int) string {
	switch {
	case blockType == blockTypeText:
		return "text"
	case blockType >= blockTypeHeading1 && blockType <= blockTypeHeading9:
		return "heading" + strconv.Itoa(blockType-blockTypeHeading1+1)
	case blockType == blockTypeBullet:
		return "bullet"
	case blockType == blockTypeOrdered:
		return "ordered"
	case blockType == blockTypeTodo:
		return "todo"
	case blockType == blockTypeCode:
		return "code"
	case blockType == blockTypeQuote:
		return "quote"
	default:
		return ""
	}
}

func childDepth(block Block, depth int) int {
	switch blockType(block) {
	case blockTypeBullet, blockTypeOrdered, blockTypeTodo:
		return depth + 1
	default:
		return depth
	}
}

func selfConsumesChildren(blockType int) bool {
	return blockType == blockTypeTable
}
