
all: build

build:
	tinygo build -target=itsybitsy-m4 -o omnivore-fw.uf2 main.go

flash:
	tinygo flash -target=itsybitsy-m4 main.go