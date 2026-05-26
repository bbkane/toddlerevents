package main

import (
	"fmt"
	"strings"

	"go.bbkane.com/warg/value/contained"
)

type bibliocommonsFeedArgs struct {
	Code string
	URL  string
}

func fromString(s string) (bibliocommonsFeedArgs, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return bibliocommonsFeedArgs{}, fmt.Errorf("invalid input: expected format 'Code,URL'")
	}

	return bibliocommonsFeedArgs{
		Code: strings.TrimSpace(parts[0]),
		URL:  strings.TrimSpace(parts[1]),
	}, nil
}

func fromIFace(i any) (bibliocommonsFeedArgs, error) {
	m, ok := i.(map[string]any)
	if !ok {
		return bibliocommonsFeedArgs{}, fmt.Errorf("invalid input: expected a map with keys 'code' and 'url'")
	}
	url, ok := m["url"].(string)
	if !ok {
		return bibliocommonsFeedArgs{}, fmt.Errorf("invalid input: 'url' must be a string")
	}
	code, ok := m["code"].(string)
	if !ok {
		return bibliocommonsFeedArgs{}, fmt.Errorf("invalid input: 'code' must be a string")
	}
	return bibliocommonsFeedArgs{
		URL:  url,
		Code: code,
	}, nil
}

func bibliocommonsTypeInfo() contained.TypeInfo[bibliocommonsFeedArgs] {
	return contained.TypeInfo[bibliocommonsFeedArgs]{
		Description: "Bibliocommons feed configuration",
		FromString:  fromString,
		FromIFace:   fromIFace,
		FromZero:    contained.FromZero[bibliocommonsFeedArgs],
		Equals:      contained.Equals[bibliocommonsFeedArgs],
	}
}
