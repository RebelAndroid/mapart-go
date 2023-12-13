package main

import (
	"encoding/csv"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
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

func make_block_choices() map[string]schematic.BlockState {
	file, err := os.Open("block_choices.csv")
	if err != nil {
		log.Fatal("an error occured in opening block_choices.csv: ", err)
	}

	reader := csv.NewReader(file)

	block_choices := map[string]schematic.BlockState{}

	for {
		record, err := reader.Read()

		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatal(err)
		}

		_, present := block_choices[record[0]]
		if present {
			log.Fatal("duplicate entry in block choices: ", record[0])
		}

		block := Block{
			id: record[1],
		}

		block_choices[record[0]] = schematic.NewBlockState(block)
	}

	return block_choices
}

func make_palette() ([]string, []color.Color, []int) {

	palette_file, err := os.Open("blockdata.csv")
	if err != nil {
		log.Fatal("an error occured in opening input.jpg: ", err)
	}

	palette_names := []string{}
	palette_colors := []color.Color{}
	palette_levels := []int{}

	palette_reader := csv.NewReader(palette_file)

	for {
		record, err := palette_reader.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatal(err)
		}

		if len(record) != 4 {
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

		// The order of left, up, and down MUST MATCH the order of the constants
		palette_colors = append(palette_colors, color.NRGBA{R: Rlevel, G: Glevel, B: Blevel, A: A})
		palette_names = append(palette_names, record[3])
		palette_levels = append(palette_levels, LEVEL)

		palette_colors = append(palette_colors, color.NRGBA{R: Rup, G: Gup, B: Bup, A: A})
		palette_names = append(palette_names, record[3])
		palette_levels = append(palette_levels, UP)

		palette_colors = append(palette_colors, color.NRGBA{R: Rdown, G: Gdown, B: Bdown, A: A})
		palette_names = append(palette_names, record[3])
		palette_levels = append(palette_levels, DOWN)
	}

	if !(len(palette_levels) == len(palette_names) && len(palette_levels) == len(palette_colors)) {
		log.Fatal("expected palette_levels, palette_names, and palette_colors lengths to be equal. ", len(palette_levels), ", ", len(palette_names), ", ", len(palette_colors))
	}

	return palette_names, palette_colors, palette_levels
}

func make_columns(img_paletted *image.Paletted) [][]uint8 {
	width := img_paletted.Rect.Size().X
	height := img_paletted.Rect.Size().Y
	stride := img_paletted.Stride
	var slices [][]uint8 = [][]uint8{}

	for x := 0; x < width; x++ {
		slices = append(slices, []uint8{})
		for y := 0; y < height; y++ {
			slices[x] = append(slices[x], img_paletted.Pix[x+y*stride])
		}
	}

	return slices
}

func make_block_states(column []uint8, palette_names []string, block_choices map[string]schematic.BlockState) []schematic.BlockState {
	block_states := []schematic.BlockState{}
	for y := 0; y < len(column); y++ {
		block_states = append(block_states, block_choices[palette_names[column[y]]])
	}

	return block_states
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
	if min_elevation < 0 {
		for i := 0; i < len(elevations); i++ {
			elevations[i] -= min_elevation
		}
	}

	for i := 0; i < len(directions); i++ {
		if directions[i] == LEVEL && elevations[i] != elevations[i+1] {
			spew.Dump(i, elevations, directions, sequences)
			log.Fatal("expected LEVEL")
		}
		if directions[i] == DOWN && elevations[i] <= elevations[i+1] {
			spew.Dump(i, elevations, directions, sequences)
			log.Fatal("expected DOWN")
		}
		if directions[i] == UP && elevations[i] >= elevations[i+1] {
			spew.Dump(i, elevations, directions, sequences)
			log.Fatal("expected UP")
		}
	}

	return elevations
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

func main() {
	input_file, err := os.Open("input.jpg")
	if err != nil {
		log.Fatal("an error occured in opening input.jpg: ", err)
	}

	img, err := jpeg.Decode(input_file)

	if err != nil {
		log.Fatal("an error occured in decoding input.jpg: ", err)
	}

	palette_names, palette_colors, _ := make_palette()

	d := dither.NewDitherer(palette_colors)
	d.Matrix = dither.FloydSteinberg

	var img_paletted *image.Paletted = d.DitherPaletted(img)

	var columns = make_columns(img_paletted)

	block_choices := make_block_choices()

	block_states := [][]schematic.BlockState{}

	for x := 0; x < len(columns); x++ {
		block_states = append(block_states, make_block_states(columns[x], palette_names, block_choices))
	}

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
		if x == 0 {
			spew.Dump(elevations[0], sequences[0], directions[0])
		}
	}

	output_file, err := os.Create("output.png")
	if err != nil {
		fmt.Println("an error occured in creating output.png: {}", err)
	}
	png.Encode(output_file, img_paletted)

	fmt.Println("hello world")
}
