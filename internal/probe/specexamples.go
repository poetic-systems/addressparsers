//go:build ignore

// specexamples.go runs every example the Project US@ specification labels
// "Correct Form" through parse.New (no reference data) and go-projectusat's
// content normalizer, and reports which ones come back unchanged, and which
// "Incorrect Form" examples reach their correct form. Run with:
//
//	go run internal/probe/specexamples.go
//
// It exists to measure go-projectusat#107's review ask ("every example
// labelled Correct in the Project US@ standard properly parsed and
// normalized") before any of it is asserted in a test. The pairs are
// transcribed from the specification's Incorrect/Correct tables, page numbers
// from the v1.0 PDF; a fragment (a street line with no number or last line)
// is run as printed and again with a last line appended, since the parser
// reads a whole address.
package main

import (
	"fmt"
	"strings"

	goprojectusat "github.com/PortobelloAuth/go-projectusat"
	"github.com/poetic-systems/addressparsers/parse"
)

type example struct {
	page      int
	incorrect string
	correct   string
}

var examples = []example{
	// Predirectional
	{16, "NORTH BAY STREET", "N BAY STREET"},
	{16, "EAST END AVE", "E END AVE"},
	// Postdirectional
	{16, "BAY DRIVE WEST", "BAY DRIVE W"},
	// Two directionals
	{16, "NORTH E MAIN STREET", "NE MAIN ST"},
	{16, "SOUTHEAST FREEWAY NORTH", "SOUTHEAST FWY N"},
	{17, "COUNTY ROAD N EAST", "COUNTY ROAD NE"},
	// Directional as part of street name
	{17, "BAY W DRIVE", "BAY WEST DRIVE"},
	{17, "NORTH AVENUE", "NORTH AVE"},
	// Street suffix as part of the name
	{19, "789 MAIN AVENUE DRIVE", "789 MAIN AVENUE DR"},
	{19, "4513 3RD STREET CIRCLE WEST", "4513 3RD STREET CIR W"},
	{19, "1000 AVE E", "1000 AVENUE E"},
	// Rural route
	{21, "RURAL ROUTE 91 BOX A7", "RR 91 BOX A7"},
	{21, "RFD 82 BOX 12", "RR 82 BOX 12"},
	{21, "RD 51 # 25", "RR 51 BOX 25"},
	{21, "RFD Route 4 #87a", "RR 4 BOX 87A"},
	{21, "RR 2 BOX 18 Bryan Dairy Rd", "RR 2 BOX 18"},
	{21, "RR03 BOX 98D", "RR 3 BOX 98D"},
	// General delivery
	{22, "GEN DELIVERY\nTAMPA, FL 33602", "GENERAL DELIVERY\nTAMPA FL 33602-9999"},
	// Post office box
	{22, "POST OFFICE BOX 11890", "PO BOX 11890"},
	{22, "POST OFFICE BOX G", "PO BOX G"},
	// Puerto Rico: apartment buildings and condominiums
	{25, "COND VERDE APT 1120", "1 COND VERDE APT 1120"},
	{25, "VISTA SUITES III APT 104", "3 VISTA SUITES APT 104"},
	// Puerto Rico: house number before the street name
	{26, "CALLE 1 A17", "A17 CALLE 1"},
	{26, "CALLE 191 B113", "13 CALLE 191"},
	{27, "CALLE 125 C-19", "C19 CALLE 125"},
	{27, "A-17 CALLE AMAPOLA", "A17 CALLE AMAPOLA"},
	{27, "B-17A CALLE 1", "B17A CALLE 1"},
	// Puerto Rico: block and house
	{27, "CALLE 19 BLQ 199 Casa 31", "199-31 CALLE 19"},
	{27, "CALLE 117 Bloque 23 Núm.18", "23-18 CALLE 117"},
	// Urbanizations
	{28, "URBANIZATION GOLDEN GATE", "URB GOLDEN GATE"},
	{28, "A17 URB JARDINES FAGOTA\nPONCE PR 00731", "A17 JARD FAGOTA\nPONCE PR 00731"},
	{29, "URB EXT VISTA BELLA", "EXT VISTA BELLA"},
	{29, "URB ALTS DE CANÁ", "ALTS DE CANA"},
	// Puerto Rico: post office box
	{29, "XYZ COMPANY\nAPARTADO 2018", "XYZ COMPANY\nPO BOX 2018"},
	{29, "ABC COMPANY\nGPO BOX 1118", "ABC COMPANY\nPO BOX 1118"},
	// Puerto Rico: postal station above the delivery line
	{30, "PO BOX 1190\nOLD SAN JUAN STA\nSAN JUAN PR 00902-1190", "OLD SAN JUAN STA\nPO BOX 1190\nSAN JUAN PR 00902-1190"},
	// Puerto Rico: rural route
	{30, "RR03 BOX 9800", "RR 3 BOX 9800"},
	{30, "RFD ROUTE 4 BZN 1725", "RR 4 BOX 1725"},
	{30, "RUTA RURAL 3 BUZON 12000", "RR 3 BOX 12000"},
	{30, "RFD 1 Bzn 17-A", "RR 1 BOX 17A"},
	{30, "RR 2 BOX 1980\nSECTOR EL BRINCO", "RR 2 BOX 1980"},
	{30, "RR 3 BOX 3415\nBARRIO VISTA ALEGRE", "RR 3 BOX 3415"},
	// Highway contract routes
	{31, "Ruta Estrella 1 Buzón 18", "HC 1 BOX 18"},
	{31, "HC 03 Bzn 1050", "HC 1 BOX 1050"},
	// Business addresses
	{35, "BIG BUSINESS INCORPORATED\n12 EAST BUSINESS LANE, SUITE-209\nKRYTON,TN\n38188-0002", "BIG BUSINESS INC\n12 E BUSINESS LN STE 209\nKRYTON, TN 38188-0022"},
	{35, "PIZZA DELIVERY COMPANY\n61-20 EAST RIVER DRIVE\nNEW YORK, NY 10021-0905", "PIZZA DELIVERY COMPANY\n61-20 E RIVER DR\nNEW YORK NY 10021-0905"},
}

// lastLine completes a fragment so the parser has a whole address to read.
const lastLine = "\nSAN JUAN PR 00907"

func main() {
	p := parse.New(parse.Options{})
	opts := []goprojectusat.USAtNormalizeOption{
		goprojectusat.WithCustomAddressParser(p),
		goprojectusat.WithContentNormalization(),
	}
	normalize := func(s string) string {
		out, err := goprojectusat.Normalize(s, opts...)
		if err != nil {
			return "ERR " + err.Error()
		}
		return out
	}
	hasLastLine := func(s string) bool { return strings.Contains(s, " PR 0") || strings.Contains(s, " FL 3") || strings.Contains(s, "TN") || strings.Contains(s, " NY 1") }

	fixed, reached := 0, 0
	fmt.Printf("%-4s %-8s %-8s %s\n", "page", "fixed", "reached", "correct → normalize(correct) | normalize(incorrect)")
	for _, e := range examples {
		correct, incorrect := e.correct, e.incorrect
		if !hasLastLine(correct) {
			correct += lastLine
			incorrect += lastLine
		}
		nc, ni := normalize(correct), normalize(incorrect)
		isFixed, isReached := nc == correct, ni == correct
		if isFixed {
			fixed++
		}
		if isReached {
			reached++
		}
		fmt.Printf("p.%-2d %-8v %-8v %q → %q | %q\n", e.page, isFixed, isReached,
			strings.TrimSuffix(e.correct, lastLine), strings.TrimSuffix(nc, lastLine), strings.TrimSuffix(ni, lastLine))
	}
	fmt.Printf("\n%d examples: %d correct forms are fixed points, %d incorrect forms reach their correct form\n", len(examples), fixed, reached)
}
