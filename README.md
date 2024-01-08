# Mapart-Go
## A program for creating Minecraft mapart
This program turns images into schematics that can be built in Minecraft to create mapart. Note that if you are building a mapart, the input image's dimensions must both be divisible by 128 and when the schematic is placed in the world it must not be rotated and aligned to a map boundary (with the northmost row of blocks, which should all be scaffolding blocks 1 block north outside the map boundary).
## Usage
Usage: mapart-go [OPTION]... [INPUT] [SCHEMATIC] [PREVIEW]
Generates a mapart schematic from INPUT, writes it to SCHEMATIC, and writes a preview png to PREVIEW
Default INPUT is 'input.png', default SCHEMATIC is 'output.litematic', default PREVIEW is output.png

Options
-s, --staircase    	currently unused, will cause the program to exit unsuccessfully
--dither=DITHER    	the dither mode to use, defaults to 'floyd-steinberg', currently available options are floyd-steinberg, false-floyd-steinberg,
	jarvis-judice-ninke, atkinson, stucki, burkes, sierra, sierra2, sierra-lite, steven-pigeon, simple-2d, or noise
--scaffold=SCAFFOLD	the block to use for scaffolding, defaults to 'cobblestone'
--strength=STRENGTH	the strength of the dither effect
