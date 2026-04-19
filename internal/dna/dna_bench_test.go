package dna_test

import (
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/dna"
)

func BenchmarkDNARecord(b *testing.B) {
	d := dna.New("bench")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Record("cpu", float64(i%100))
	}
}

func BenchmarkDNAIsAnomaly(b *testing.B) {
	d := dna.New("bench")
	for i := 0; i < 1000; i++ {
		d.Record("cpu", 45.0)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.IsAnomaly("cpu", 95.0)
	}
}

func BenchmarkDNAHealthScore(b *testing.B) {
	d := dna.New("bench")
	for i := 0; i < 1000; i++ {
		d.Record("cpu", 45.0)
		d.Record("mem", 60.0)
	}
	current := map[string]float64{"cpu": 50.0, "mem": 65.0}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.HealthScore(current)
	}
}
