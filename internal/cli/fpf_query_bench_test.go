package cli

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
)

// Разбивка стоимости одного FPF-запроса. Замер живого MCP-сервера даёт 262 мс
// на установившемся режиме; эти бенчмарки разделяют её на три части, чтобы
// кэшировать дорогое, а не предполагаемое.
//
// Временный диагностический файл. Удалить после того, как кэш измерен.

// BenchmarkFPFOpenOnly — только распаковка встроенной базы во временный файл
// и открытие handle, без запроса.
func BenchmarkFPFOpenOnly(b *testing.B) {
	for b.Loop() {
		db, cleanup, err := openFPFDBFunc()
		if err != nil {
			b.Fatal(err)
		}
		cleanup()
		_ = db
	}
}

// BenchmarkFPFSnapshotOnly — открытие плюс LoadQuerySourceSnapshot, без поиска.
func BenchmarkFPFSnapshotOnly(b *testing.B) {
	for b.Loop() {
		db, cleanup, err := openFPFDBFunc()
		if err != nil {
			b.Fatal(err)
		}
		if _, err := fpf.LoadQuerySourceSnapshot(db); err != nil {
			cleanup()
			b.Fatal(err)
		}
		cleanup()
	}
}

// BenchmarkFPFConcernQuery — полный путь, который и платит агент.
func BenchmarkFPFConcernQuery(b *testing.B) {
	request := fpf.ConcernQuery{Text: "how to compare alternatives under parity"}
	for b.Loop() {
		if _, err := queryEmbeddedFPF(request); err != nil {
			b.Fatal(err)
		}
	}
}
