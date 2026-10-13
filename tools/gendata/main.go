// Command gendata builds the embedded place database from GeoNames dumps.
//
//	curl -LO https://download.geonames.org/export/dump/cities15000.zip
//	curl -LO https://download.geonames.org/export/dump/countryInfo.txt
//	curl -LO https://download.geonames.org/export/dump/admin1CodesASCII.txt
//	unzip cities15000.zip
//	go run ./tools/gendata -src <dir> -out internal/places/data
//
// GeoNames data is licensed under CC BY 4.0 (https://www.geonames.org).
package main

import (
	"bufio"
	"compress/gzip"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Alternate names are only kept for big cities, so "Bombay" or "NYC" resolve
// without bloating the database with every transliteration of every town.
const altMinPopulation = 1_000_000

type city struct {
	name, ascii, cc, adminCode, admin, tz string
	pop                                   int
	alts                                  []string
}

func main() {
	src := flag.String("src", "", "directory with cities15000.txt, countryInfo.txt, admin1CodesASCII.txt")
	out := flag.String("out", "internal/places/data", "output directory")
	flag.Parse()
	if *src == "" {
		log.Fatal("-src is required")
	}

	admin1 := map[string]string{}
	eachRow(filepath.Join(*src, "admin1CodesASCII.txt"), func(f []string) {
		if len(f) >= 2 {
			admin1[f[0]] = f[1]
		}
	})

	var cities []city
	eachRow(filepath.Join(*src, "cities15000.txt"), func(f []string) {
		if len(f) < 18 || f[17] == "" {
			return
		}
		pop, _ := strconv.Atoi(f[14])
		c := city{
			name:      f[1],
			ascii:     f[2],
			cc:        f[8],
			adminCode: f[10],
			admin:     admin1[f[8]+"."+f[10]],
			tz:        f[17],
			pop:       pop,
		}
		if pop >= altMinPopulation {
			c.alts = asciiAlternates(f[3], c.name, c.ascii)
		}
		cities = append(cities, c)
	})
	sort.Slice(cities, func(i, j int) bool { return cities[i].pop > cities[j].pop })

	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}

	cf, err := os.Create(filepath.Join(*out, "cities.tsv.gz"))
	if err != nil {
		log.Fatal(err)
	}
	gz, _ := gzip.NewWriterLevel(cf, gzip.BestCompression)
	w := bufio.NewWriter(gz)
	for _, c := range cities {
		ascii := c.ascii
		if ascii == c.name {
			ascii = ""
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			c.name, ascii, c.cc, c.adminCode, c.admin, c.pop, c.tz, strings.Join(c.alts, "|"))
	}
	must(w.Flush())
	must(gz.Close())
	must(cf.Close())

	kf, err := os.Create(filepath.Join(*out, "countries.tsv"))
	if err != nil {
		log.Fatal(err)
	}
	kw := bufio.NewWriter(kf)
	eachRow(filepath.Join(*src, "countryInfo.txt"), func(f []string) {
		if len(f) < 6 || strings.HasPrefix(f[0], "#") {
			return
		}
		// iso2, iso3, name, capital
		fmt.Fprintf(kw, "%s\t%s\t%s\t%s\n", f[0], f[1], f[4], f[5])
	})
	must(kw.Flush())
	must(kf.Close())

	log.Printf("wrote %d cities to %s", len(cities), *out)
}

func asciiAlternates(field, name, ascii string) []string {
	seen := map[string]bool{strings.ToLower(name): true, strings.ToLower(ascii): true}
	var alts []string
	for _, a := range strings.Split(field, ",") {
		a = strings.TrimSpace(a)
		if len(a) < 3 || !isPlainASCII(a) || seen[strings.ToLower(a)] {
			continue
		}
		seen[strings.ToLower(a)] = true
		alts = append(alts, a)
	}
	return alts
}

func isPlainASCII(s string) bool {
	for _, r := range s {
		if !(r == ' ' || r == '.' || r == '\'' || r == '-' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return true
}

func eachRow(path string, fn func([]string)) {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		fn(strings.Split(sc.Text(), "\t"))
	}
	must(sc.Err())
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
