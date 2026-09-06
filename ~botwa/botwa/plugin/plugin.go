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
    if len(args) == 0 {
        return
    }

    // Ambil command (hapus prefix di karakter pertama)
    cmdWithPrefix := args[0]

    var cmd string
    var params []string

if len(cmdWithPrefix) > 1 {
    // Format normal: ".menu"
    cmd = cmdWithPrefix[1:]
    params = args[1:]
} else if len(args) > 1 {
    // Format dengan spasi: ". menu"
    cmd = args[1]
    params = args[2:]
} else {
    return
}

    if p, ok := Plugins[cmd]; ok {
        p.Run(client, m, params)
    }
}