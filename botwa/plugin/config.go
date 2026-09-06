package plugin

// ownerNumbers diisi lewat SetOwnerNumbers() dari main.go, nilainya diambil
// dari OwnerNumbers di config.go (package main). Dibuat begini (bukan
// langsung import) supaya package plugin tidak import package main —
// itu akan bikin import cycle karena main.go sudah import "botwa/plugin".
var ownerNumbers []string

// SetOwnerNumbers dipanggil sekali di main.go saat bot start.
func SetOwnerNumbers(numbers []string) {
    ownerNumbers = numbers
}

// pairingCode diisi lewat SetPairingCode() dari main.go, nilainya diambil
// dari PairingCode di config.go (package main). Dipakai baik oleh bot utama
// (main.go) maupun plugin jadibot ini, supaya SEMUA sesi (utama & jadibot)
// pakai kode pairing custom yang sama.
var pairingCode string

// SetPairingCode dipanggil sekali di main.go saat bot start.
func SetPairingCode(code string) {
    pairingCode = code
}