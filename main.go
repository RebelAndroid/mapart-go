package main

import (
	"encoding/csv"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/makeworld-the-better-one/dither/v2"

	"github.com/elvis972602/go-litematica-tools/schematic"
)

const (
	LEVEL = iota // the same level as the block to the north
	UP           // higher level than the block to the north
	DOWN         // lower level than the block to the north
)

type Sequence struct {
	direction int
	height    int
	length    int
}

type Block struct {
	id string
}

func (b Block) ID() string {
	return b.id
}

type PaletteColor struct {
	color    color.Color
	name     string
	block    Block
	scaffold bool
}

func make_palette() []PaletteColor {

	palette_file, err := os.Open("blockdata.csv")
	if err != nil {
		log.Fatal("an error occured in opening blockdata.csv: ", err)
	}

	palette := []PaletteColor{}

	palette_reader := csv.NewReader(palette_file)

	for {
		record, err := palette_reader.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatal(err)
		}

		if len(record) != 6 {
			log.Fatal("unexpected number of values in blockdata.csv:", len(record))
		}
		R, _ := strconv.Atoi(record[0])
		G, _ := strconv.Atoi(record[1])
		B, _ := strconv.Atoi(record[2])
		Rlevel := uint8(R * 220 / 255)
		Glevel := uint8(G * 220 / 255)
		Blevel := uint8(B * 220 / 255)
		Rup := uint8(R)
		Gup := uint8(G)
		Bup := uint8(B)
		Rdown := uint8(R * 180 / 255)
		Gdown := uint8(G * 180 / 255)
		Bdown := uint8(B * 180 / 255)
		A := uint8(255)

		sc_str := strings.ToLower(strings.Trim(record[5], "\t "))
		scaffold := sc_str == "true"
		if (!scaffold) && (sc_str != "false") {
			log.Fatal("unexpected scaffold value (should be \"true\" or \"false\"): ", record[5])
		}

		// must be in the same order as the "enum"
		palette = append(palette, PaletteColor{color: color.NRGBA{R: Rlevel, G: Glevel, B: Blevel, A: A}, name: record[3] + "LEVEL", block: Block{record[4]}, scaffold: scaffold})
		palette = append(palette, PaletteColor{color: color.NRGBA{R: Rup, G: Gup, B: Bup, A: A}, name: record[3] + "UP", block: Block{record[4]}, scaffold: scaffold})
		palette = append(palette, PaletteColor{color: color.NRGBA{R: Rdown, G: Gdown, B: Bdown, A: A}, name: record[3] + "DOWN", block: Block{record[4]}, scaffold: scaffold})
	}

	return palette
}

func make_color_palette(palette []PaletteColor) []color.Color {
	colors := []color.Color{}

	for i := 0; i < len(palette); i++ {
		colors = append(colors, palette[i].color)
	}

	return colors
}

func make_columns(img_paletted *image.Paletted, palette []PaletteColor) [][]uint8 {
	width := img_paletted.Rect.Size().X
	height := img_paletted.Rect.Size().Y
	var slices [][]uint8 = [][]uint8{}

	for x := 0; x < width; x++ {
		slices = append(slices, []uint8{})
		for y := 0; y < height; y++ {
			index := img_paletted.ColorIndexAt(x, y)
			r1, g1, b1, a1 := img_paletted.At(x, y).RGBA()
			r2, g2, b2, a2 := palette[index].color.RGBA()

			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				spew.Dump(index, img_paletted.At(x, y), palette[index])
				log.Fatal("internal error")
			}
			slices[x] = append(slices[x], index)
		}
	}

	return slices
}

func make_elevations(sequences []Sequence, directions []int) []int {
	// create elevations with initial dummy block
	elevations := []int{0}

	for i := 0; i < len(sequences); i++ {
		if sequences[i].direction == LEVEL {
			if len(sequences) != 1 {
				log.Fatal("column starting with level sequence should not have multiple sequences")
			}
			// a flat level at zero, length is plus one for the dummy block at the start
			for y := 0; y < len(directions); y++ {
				elevations = append(elevations, 0)
			}
		} else if sequences[i].direction == UP {
			for j := 0; j < sequences[i].length; j++ {
				direction := directions[len(elevations)-1]
				if direction == LEVEL {
					elevations = append(elevations, elevations[len(elevations)-1])
				} else if direction == UP {
					elevations = append(elevations, elevations[len(elevations)-1]+1)
				} else {
					log.Fatal("unexpected DOWN in UP sequence")
				}
			}
		} else if sequences[i].direction == DOWN {
			for j := 0; j < sequences[i].length; j++ {
				direction := directions[len(elevations)-1]
				if direction == LEVEL {
					elevations = append(elevations, elevations[len(elevations)-1])
				} else if direction == DOWN {
					elevations = append(elevations, elevations[len(elevations)-1]-1)
				} else {
					spew.Dump(elevations)
					log.Fatal("unexpected UP in DOWN sequence at ")
				}
			}
		}
	}
	min_elevation := 1000000
	for i := 0; i < len(elevations); i++ {
		min_elevation = min(min_elevation, elevations[i])
	}

	// if any portion of the structure goes to low, raise the entire column so it fits
	// we want the bottom elevation to be at 1 to leave room for a scaffolding block below it
	if min_elevation < 1 {
		for i := 0; i < len(elevations); i++ {
			elevations[i] -= min_elevation - 1
		}
	}

	if !test_elevations(elevations, directions) {
		log.Fatal("error generating elevations")
	}

	return elevations
}

func test_elevations(elevations []int, directions []int) bool {
	if len(elevations) != len(directions)+1 {
		return false
	}
	for i := 0; i < len(directions); i++ {
		if elevations[i] < 1 || elevations[i] > 255 {
			return false
		}
		if directions[i] == LEVEL && elevations[i] != elevations[i+1] {
			return false
		}
		if directions[i] == DOWN && elevations[i] <= elevations[i+1] {
			return false
		}
		if directions[i] == UP && elevations[i] >= elevations[i+1] {
			return false
		}
	}
	return true
}

func make_sequences(directions []int) []Sequence {
	sequences := []Sequence{}

	height := 0
	length := 0
	direction := LEVEL

	for i := 0; i < len(directions); i++ {
		if directions[i] == LEVEL {
			length++
		} else if directions[i] != direction {
			if direction == LEVEL {
				direction = directions[i]
				length++
				height++
			} else {
				sequences = append(sequences, Sequence{
					direction: direction,
					height:    height,
					length:    length,
				})
				height = 1
				length = 1
				direction = directions[i]
			}
		} else {
			length++
			height++
		}
	}

	sequences = append(sequences, Sequence{
		direction: direction,
		height:    height,
		length:    length,
	})

	return sequences
}

func make_directions(column []uint8) []int {
	directions := []int{}

	for y := 0; y < len(column); y++ {
		directions = append(directions, int(column[y])%3)
	}

	return directions
}

func color_equal(a color.Color, b color.Color) bool {
	r1, g1, b1, a1 := a.RGBA()
	r2, g2, b2, a2 := b.RGBA()

	return r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2
}

func make_schematic(elevations [][]int, palette []PaletteColor, img_paletted image.Paletted, scaffold_block Block) schematic.Project {
	project := schematic.NewProject("mapart", len(elevations), 256, len(elevations[0]))

	for x := 0; x < len(elevations); x++ {
		for y := 0; y < len(elevations[0]); y++ {
			block := palette[img_paletted.ColorIndexAt(x, y-1)].block
			// one is added to all elevations to make space for scaffolding blocks
			project.SetBlock(x, elevations[x][y]+1, y, block)

			if palette[img_paletted.ColorIndexAt(x, y-1)].scaffold {
				project.SetBlock(x, elevations[x][y], y, scaffold_block)
			}
		}
	}

	return *project
}

type Args struct {
	input_file         string
	preview_location   string
	schematic_location string
	staircase          bool
	dither_type        string
}

func parse() Args {
	args := Args{
		input_file:         "",
		schematic_location: "",
		preview_location:   "",
		staircase:          false,
		dither_type:        "",
	}

	for i := 1; i < len(os.Args); i++ {
		if strings.HasPrefix(os.Args[i], "--") {
			if args.input_file != "" {
				log.Println("options should be placed before arguments")
			}
			if os.Args[i] == "--staircase" {
				log.Fatal("unimplemented!")
				if args.staircase {
					log.Println("staircase specified twice")
				}
				args.staircase = true
				continue
			}
			mode, dither := strings.CutPrefix(os.Args[i], "--dither=")
			if dither {
				args.dither_type = mode
				continue
			}
			log.Fatal("unknown option: ", os.Args[i])
		}
		if strings.HasPrefix(os.Args[i], "-") {
			if args.input_file != "" {
				log.Println("options should be placed before arguments")
			}
			if os.Args[0] == "-s" {
				log.Fatal("unimplemented!")
				if args.staircase {
					log.Println("staircase specified twice")
				}
				args.staircase = true
				continue
			}
		}
		if args.input_file == "" {
			args.input_file = os.Args[i]
			continue
		}
		if args.schematic_location == "" {
			if !strings.HasSuffix(os.Args[i], ".litematic") {
				log.Fatal("Output location should be a .litematic file! Found: ", os.Args[i])
			}
			args.schematic_location = os.Args[i]
			continue
		}
		if args.preview_location == "" {
			if !strings.HasSuffix(os.Args[i], ".png") {
				log.Fatal("Output location should be a .png file! Found: ", os.Args[i])
			}
			args.preview_location = os.Args[i]
			continue
		}
		log.Fatal("too many arguments")
	}

	if args.input_file == "" {
		args.input_file = "input.png"
	}

	if args.schematic_location == "" {
		args.schematic_location = "output.litematic"
	}

	if args.preview_location == "" {
		args.preview_location = "output.png"
	}

	if args.dither_type == "" {
		args.dither_type = "floyd-steinberg"
	}

	return args
}

func main() {
	args := parse()

	input_file, err := os.Open(args.input_file)
	if err != nil {
		log.Fatal("an error occured in opening input image: ", err)
	}

	img, _, err := image.Decode(input_file)

	if err != nil {
		log.Fatal("an error occured in decoding input image: ", err)
	}

	palette := make_palette()
	color_palette := make_color_palette(palette)

	d := dither.NewDitherer(color_palette)
	if args.dither_type == "floyd-steinberg" {
		d.Matrix = dither.FloydSteinberg
	} else if args.dither_type == "stucki" {
		d.Matrix = dither.Stucki
	} else {
		log.Fatal("Unknown dither type: ", args.dither_type, "\nValid dither types are: floyd-steinberg, stucki")
	}

	var img_paletted *image.Paletted = d.DitherPaletted(img)

	var columns = make_columns(img_paletted, palette)

	directions := [][]int{}

	for x := 0; x < len(columns); x++ {
		directions = append(directions, make_directions(columns[x]))
	}

	sequences := [][]Sequence{}

	for x := 0; x < len(columns); x++ {
		sequences = append(sequences, make_sequences(directions[x]))
	}

	elevations := [][]int{}

	for x := 0; x < len(columns); x++ {
		elevations = append(elevations, make_elevations(sequences[x], directions[x]))
	}

	project := make_schematic(elevations, palette, *img_paletted, Block{id: "minecraft:stone"})

	schematic_output_file, err := os.Create(args.schematic_location)
	if err != nil {
		log.Fatal("an error occured in creating output schematic: ", err)
	}
	project.Encode(schematic_output_file)

	preview_output_file, err := os.Create(args.preview_location)
	if err != nil {
		log.Fatal("an error occured in creating output preview: ", err)
	}
	png.Encode(preview_output_file, img_paletted)

	fmt.Println("finished")
}
