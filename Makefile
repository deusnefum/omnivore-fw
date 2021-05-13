OPTIONS=-scheduler=coroutines -target=itsybitsy-m4
#OPTIONS=-scheduler=tasks -target=itsybitsy-m4 -

all: build

build:
	/home/zashi/tmp/tinygo/build/tinygo build ${OPTIONS} -o omnivore-fw.uf2 main.go

flash:
	tinygo flash ${OPTIONS} main.go