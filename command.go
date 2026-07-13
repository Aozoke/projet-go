package main

type CommandType string

const (
	CommandSet    CommandType = "SET"
	CommandGet    CommandType = "GET"
	CommandDelete CommandType = "DELETE"
)

type Command struct {
	Type  CommandType
	Key   string
	Value string
}
