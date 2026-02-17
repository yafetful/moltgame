ffmpeg -i input.mp4 -vf "chromakey=0x00ff00:0.3:0.1" -c:v libvpx-vp9 -pix_fmt yuva444p -b:v 2M -an output.webm

ffmpeg -i input.mp4 -vf "chromakey=0x00ff40:0.3:0.1"  -c:v libvpx-vp9 -pix_fmt yuva444p -b:v 0 -crf 15 -row-mt 1 -an output.webm

ffmpeg -i input.mp4 \  -vf "chromakey=0x00ff00:0.3:0.1,scale=720:-1" \  -c:v libvpx-vp9 \  -pix_fmt yuva444p \  -b:v 0 -crf 24 \  -row-mt 1 \  -an output.webm
