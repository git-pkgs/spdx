package spdx

import (
	"encoding/json"
	"os"
	"sort"
	"testing"
)

var (
	benchmarkNormalized string
	benchmarkExpression Expression
	benchmarkErr        error
	benchmarkValid      bool
)

func BenchmarkNormalizeCanonicalIdentifiers(b *testing.B) {
	benchmarkNormalizeCorpus(b, []string{
		"MIT",
		"Apache-2.0",
		"BSD-3-Clause",
		"ISC",
		"MPL-2.0",
		"GPL-3.0-only",
		"LGPL-2.1-or-later",
		"CC0-1.0",
		"Unlicense",
		"Zlib",
	})
}

func BenchmarkNormalizeCommonInformalNames(b *testing.B) {
	benchmarkNormalizeCorpus(b, []string{
		"MIT License",
		"Apache 2.0",
		"GPL v3",
		"GNU General Public License v2",
		"BSD 3-Clause",
		"Mozilla Public License 2.0",
		"Eclipse Public License 2.0",
		"Public Domain",
		"CC BY 4.0",
		"Boost",
	})
}

func BenchmarkNormalizeInvalidValues(b *testing.B) {
	benchmarkNormalizeCorpus(b, []string{
		"",
		"UNKNOWN-LICENSE",
		"FAKEYLICENSE",
		"NOT-A-LICENSE",
		"commercial proprietary terms",
		"license information unavailable",
	})
}

func BenchmarkNormalizeExpressions(b *testing.B) {
	expressions := []string{
		"MIT",
		"MIT OR Apache-2.0",
		"GPL-2.0-only WITH Classpath-exception-2.0",
		"MIT OR GPL-2.0-only AND Apache-2.0",
		"(Apache 2 OR MIT License) AND GPL v3",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, expression := range expressions {
			benchmarkNormalized, benchmarkErr = NormalizeExpression(expression)
		}
	}
}

func BenchmarkNormalizeRepeatedValues(b *testing.B) {
	inputs := make([]string, 1000)
	for i := range inputs {
		inputs[i] = "Apache License 2.0"
	}
	benchmarkNormalizeCorpus(b, inputs)
}

func BenchmarkNormalizeMostlyUniqueRealWorld(b *testing.B) {
	data, err := os.ReadFile("real_licenses.json")
	if err != nil {
		b.Skip("real_licenses.json not found")
	}

	licenses := make(map[string]int)
	if err := json.Unmarshal(data, &licenses); err != nil {
		b.Fatalf("parse real-world licenses: %v", err)
	}

	inputs := make([]string, 0, len(licenses))
	for license := range licenses {
		inputs = append(inputs, license)
	}
	sort.Strings(inputs)
	benchmarkNormalizeCorpus(b, inputs)
}

func BenchmarkNormalizePaths(b *testing.B) {
	benchmarks := []struct {
		name  string
		input string
	}{
		{name: "ExactMatch", input: "Apache-2.0"},
		{name: "Transform", input: "Apache 2.0"},
		{name: "Transposition", input: "MIT License"},
		{name: "Fallback", input: "licensed under the BSD terms"},
	}

	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				benchmarkNormalized, benchmarkErr = Normalize(benchmark.input)
			}
		})
	}
}

func BenchmarkParseCanonicalExpressions(b *testing.B) {
	expressions := []string{
		"MIT",
		"MIT OR Apache-2.0",
		"GPL-2.0-only WITH Classpath-exception-2.0",
		"MIT OR (GPL-2.0-only AND Apache-2.0)",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, expression := range expressions {
			benchmarkExpression, benchmarkErr = Parse(expression)
		}
	}
}

func BenchmarkValidateExpressions(b *testing.B) {
	expressions := []string{
		"MIT",
		"MIT OR Apache-2.0",
		"GPL-2.0-only WITH Classpath-exception-2.0",
		"MIT OR (GPL-2.0-only AND Apache-2.0)",
		"invalid-license",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, expression := range expressions {
			benchmarkValid = Valid(expression)
		}
	}
}

func benchmarkNormalizeCorpus(b *testing.B, inputs []string) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	b.ReportMetric(float64(len(inputs)), "inputs/op")
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			benchmarkNormalized, benchmarkErr = Normalize(input)
		}
	}
}
