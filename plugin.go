package plugin

import (
    "strings"

    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/types/events"
)

type Plugin struct {
    Command string
    Desc    string
    Run     func(*whatsmeow.Client, *events.Message, []string)
}

var Plugins = map[string]Plugin{}

func Register(p Plugin) {
    Plugins[p.Command] = p
}

func Init() {}

func Execute(client *whatsmeow.Client, m *events.Message, text string) {
    args := strings.Fields(text)
    cmd := strings.TrimPrefix(args[0], "!")
    params := args[1:]

    if p, ok := Plugins[cmd]; ok {
        p.Run(client, m, params)
    }
}
