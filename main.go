package main

import (
	"encoding/csv"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"os"
	"strconv"

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
	color color.Color
	name  string
	block Block
}

func make_palette() []PaletteColor {

	palette_file, err := os.Open("blockdata.csv")
	if err != nil {
		log.Fatal("an error occured in opening input.jpg: ", err)
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

		if len(record) != 5 {
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

		// must be in the same order as the "enum"
		palette = append(palette, PaletteColor{color: color.NRGBA{R: Rlevel, G: Glevel, B: Blevel, A: A}, name: record[3] + "LEVEL", block: Block{record[4]}})
		palette = append(palette, PaletteColor{color: color.NRGBA{R: Rup, G: Gup, B: Bup, A: A}, name: record[3] + "UP", block: Block{record[4]}})
		palette = append(palette, PaletteColor{color: color.NRGBA{R: Rdown, G: Gdown, B: Bdown, A: A}, name: record[3] + "DOWN", block: Block{record[4]}})
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

func make_schematic(elevations [][]int, palette []PaletteColor, img_paletted image.Paletted) schematic.Project {
	project := schematic.NewProject("mapart", len(elevations), 256, len(elevations[0]))

	for x := 0; x < len(elevations); x++ {
		for y := 0; y < len(elevations[0]); y++ {
			// this automatically adds a dummy block of ID 0
			block := palette[img_paletted.ColorIndexAt(x, y-1)].block
			project.SetBlock(x, elevations[x][y], y, block)
		}
	}

	return *project
}

func main() {

	if len(os.Args) < 2 {
		log.Fatal("expected at least command line argument: input filename")
	}
	input_filename := os.Args[1]

	input_file, err := os.Open(input_filename)
	if err != nil {
		log.Fatal("an error occured in opening input.jpg: ", err)
	}

	img, _, err := image.Decode(input_file)

	if err != nil {
		log.Fatal("an error occured in decoding input.jpg: ", err)
	}

	palette := make_palette()
	color_palette := make_color_palette(palette)

	d := dither.NewDitherer(color_palette)
	d.Matrix = dither.FloydSteinberg

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

	project := make_schematic(elevations, palette, *img_paletted)

	schematic_output_file, err := os.Create("output.litematic")
	if err != nil {
		log.Fatal("an error occured in creating output.litematic: ", err)
	}
	project.Encode(schematic_output_file)

	preview_output_file, err := os.Create("output.png")
	if err != nil {
		log.Fatal("an error occured in creating output.png: ", err)
	}
	png.Encode(preview_output_file, img_paletted)

	fmt.Println("finished")
}
