
all: build

build:
	tinygo build -target=itsybitsy-m4 -o fw.bin main.go