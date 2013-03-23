package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Printf("usage: %v <path.to.iso3.json> <import-export-file.csv>\n", os.Args[0])
		return
	}

	country := path.Base(os.Args[2])
	if pos := strings.Index(country, "_"); pos > -1 {
		country = country[:pos]
	}

	buf, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	countryCodes := make([][]string, 0) // [["CH", "CHE"], ...]
	if err := json.Unmarshal(buf, &countryCodes); err != nil {
		fmt.Printf("%v\n", err)
	}

	iso3 := make(map[string]string)
	for _, cc := range countryCodes {
		iso3[cc[0]] = cc[1]
	}

	buf, err = ioutil.ReadFile(os.Args[2])
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	take := false
	for _, line := range strings.Split(string(buf), "\r") {
		if strings.Index(line, "By main destination") > -1 {
			take = true
			continue
		}
		if strings.HasPrefix(line, ";") {
			take = false
		}

		if take {
			var cells []string
			for _, cell := range strings.Split(line, ";") {
				cell = strings.Trim(cell, " \t\r\n")
				if cell != "" {
					cells = append(cells, cell)
				}
			}

			switch len(cells) {
			case 4:
				if err := printRecord(iso3, country, cells[0], cells[1], true); err != nil {
					fmt.Printf("%v\n", err)
					return
				}

				if err := printRecord(iso3, country, cells[2], cells[3], false); err != nil {
					fmt.Printf("%v\n", err)
					return
				}
			case 2:
				if err := printRecord(iso3, country, cells[0], cells[1], true); err != nil {
					fmt.Printf("%v\n", err)
					return
				}
			default:
				fmt.Printf("invalid record: '%v'\n", line)
				return
			}
		}
	}
}

func printRecord(iso3 map[string]string, country1, country2, percent string, export bool) error {
	country1 = iso3[country1]
	if country1 == "" {
		return fmt.Errorf("unknown iso2 code: %v", country1)
	}

	// cut the "1. " and ignore "...", "unspecified destinations"
	country2 = country2[3:]
	if country2 == "..." {
		return nil
	}
	if pos := strings.Index(country2, ","); pos > -1 {
		country2 = country2[:pos]
	}
	if strings.ToLower(country2) == "pecified destinations" {
		return nil
	}
	if strings.ToLower(country2) == "pecified origins" {
		return nil
	}
	if strings.ToLower(country2) == "european union (27)" {
		country2 = "E27"
	} else {
		var err error
		country2, err = lookupISO3(iso3, country2)
		if err != nil {
			return err
		}
	}

	if export {
		fmt.Printf("%v,%v,%v\n", country1, country2, percent)
	} else {
		fmt.Printf("%v,%v,%v\n", country2, country1, percent)
	}

	return nil
}

func lookupISO3(iso3 map[string]string, name string) (string, error) {
	// lookup the iso2 code by country name
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	req.Query = fmt.Sprintf(`[ g.countryCode | g <- geonames, g.featureCode == "PCLI" && (g.ascii == "%v" || g.altnames =~ "%v") ]`, name, name)
	req.Limit = 1

	reqBuf, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	resp, err := http.Post("https://data.mingle.io/", "application/json", bytes.NewReader(reqBuf))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var obj struct {
		Body [][]string `json:"body"`
	}
	if err := json.Unmarshal(buf, &obj); err != nil {
		return "", err
	}

	if len(obj.Body) != 1 || len(obj.Body[0]) != 1 {
		return "", fmt.Errorf("unknown country: '%v', resp: %v\n", name, string(buf))
	}

	// convert iso3 to iso2
	iso2 := obj.Body[0][0]
	if iso3[iso2] == "" {
		return "", fmt.Errorf("no iso3 mapping found for '%v' (%v)", name, iso2)
	}

	return iso3[iso2], nil
}
