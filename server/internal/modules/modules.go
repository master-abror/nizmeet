// Package modules adalah satu-satunya tempat modul server didaftarkan
// (ADR-0003, butir 1). Menambah modul = folder baru di internal/<modul>
// ditambah satu entri di All.
package modules

import "github.com/master-abror/nizmeet/server/internal/module"

// All mengembalikan semua modul sesuai urutan registrasi.
func All() []module.Module {
	return []module.Module{}
}
