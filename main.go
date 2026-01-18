package main

import "flag"

func main() {

	outPath := flag.String("out", "./", "Path to output xml [default=./]")
	flag.Parse()

	runTvSporedi(*outPath)

	//runT2(*outPath)

	//runSiol(*outPath)

	//runA1(*outPath)

}
