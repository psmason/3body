go run lissajous.go \
    | ffmpeg -f image2pipe -pix_fmt yuv420p -r 8 -i - -f ogg -qscale:v 10 -f ogg - \
    | ffplay -
