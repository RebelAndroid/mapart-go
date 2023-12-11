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

	"github.com/makeworld-the-better-one/dither/v2"

	"github.com/elvis972602/go-litematica-tools/schematic"
)

const (
	LEVEL = iota // the same level as the block to the north
	UP           // higher level than the block to the north
	DOWN         // lower level than the block to the north
)

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

	if len(slices) != width {
		log.Fatal("slices length should be equal to width ", len(slices), width)
	}

	for x := 0; x < width; x++ {
		if len(slices[x]) != height {
			log.Fatal("all slices[x] should be equal to height")
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

func make_elevations(directions []int) []int {
	return []int{}
}

struct Sequnce {
	
}

func make_sequences(directions []int) []struct{ int int } {

	sequences := []struct {
		int
		int
	}{}

	sequences = append(sequences, struct {
		int
		int
	}{0, 0})

	direction := directions[0]
	length := 0
	for x := 0; x < len(directions); x++ {
		if direction != directions[x] {
			sequences = append(sequences, struct{ int int }{direction, length})
			direction = directions[x]
			length = 0
		}
	}

	return sequences
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

	output_file, err := os.Create("output.png")
	if err != nil {
		fmt.Println("an error occured in creating output.png: {}", err)
	}
	png.Encode(output_file, img_paletted)

	fmt.Println("hello world")
}
