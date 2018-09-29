go run lissajous.go | ffmpeg -f image2pipe -pix_fmt yuv420p -r 24 -i - -f matroska - | ffplay -
