module omnivore-fw

go 1.16

require (
	ppm v0.0.0
	dshot v0.0.0
	tinygo.org/x/drivers v0.15.1 // indirect
)

replace dshot => ./dshot
replace ppm => ./ppm
