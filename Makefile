#OPTIONS=-scheduler=coroutines -target=itsybitsy-m4
OPTIONS=-target=nano-rp2040

all: build

build:
	tinygo build ${OPTIONS} -o omnivore-fw.uf2

flash: build
	sudo mount `lsblk -o PATH,LABEL | grep RPI-RP2 | cut -f1 -d' '` /mnt -o uid=zashi
	@read -p "remove the wire"
	cp omnivore-fw.uf2 /mnt
	sudo umount /mnt