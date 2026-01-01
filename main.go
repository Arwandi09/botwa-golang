package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "botwa/plugin"

    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/store/sqlstore"
    "go.mau.fi/whatsmeow/types/events"
    waLog "go.mau.fi/whatsmeow/util/log"
    _ "github.com/mattn/go-sqlite3"
)

func main() {
    ctx := context.Background()

    // logger kosong (biar tidak berisik)
    dbLog := waLog.Noop

    // ✅ API BARU: sqlstore.New BUTUH context
    store, err := sqlstore.New(
        ctx,
        "sqlite3",
        "file:session.db?_foreign_keys=on",
        dbLog,
    )
    if err != nil {
        log.Fatal(err)
    }

    // ✅ API BARU: GetFirstDevice BUTUH context
    device, err := store.GetFirstDevice(ctx)
    if err != nil {
        log.Fatal(err)
    }

    client := whatsmeow.NewClient(device, waLog.Noop)

    // handler pesan
    client.AddEventHandler(Handler(client))

    // channel untuk nunggu QR event (WAJIB walau pairing code)
    qrReady := make(chan struct{})

    client.AddEventHandler(func(evt interface{}) {
        switch evt.(type) {
        case *events.QR:
            select {
            case <-qrReady:
            default:
                close(qrReady)
            }
        }
    })

    // CONNECT DULU
    if err := client.Connect(); err != nil {
        log.Fatal(err)
    }

    // tunggu QR event atau delay aman
    select {
    case <-qrReady:
    case <-time.After(2 * time.Second):
    }

    // JIKA BELUM LOGIN → PAIRING CODE
    if client.Store.ID == nil {
        code, err := client.PairPhone(
            ctx,
            PairingNumber,
            true,
            whatsmeow.PairClientChrome,
            "Chrome (Linux)",
        )
        if err != nil {
            log.Fatal(err)
        }

        fmt.Println("================================")
        fmt.Println(" PAIRING CODE :", code)
        fmt.Println("================================")
        fmt.Println("WhatsApp → Perangkat tertaut → Gunakan kode")
    }

    plugin.Init()

    // tahan program
    select {}
}