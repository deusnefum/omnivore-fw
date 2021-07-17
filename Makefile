OPTIONS=-scheduler=coroutines -target=itsybitsy-m4
#OPTIONS=-scheduler=tasks -target=itsybitsy-m4

all: build

build:
	tinygo build ${OPTIONS} -o omnivore-fw.uf2 main.go

flash: build
	sudo mount `lsblk -o PATH,LABEL | grep ITSYM4BOOT | cut -f1 -d' '` /mnt -o uid=zashi
	cp omnivore-fw.uf2 /mnt
	sudo umount /mnt