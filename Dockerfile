FROM archlinux:latest
WORKDIR /app
COPY . ./
RUN pacman -Syu --noconfirm
RUN pacman -S --noconfirm go git base-devel mesa glad libxrender libva
RUN go mod download
RUN go build -o smoothpaper cmd/smoothpaper/smoothpaper.go
RUN chmod +x smoothpaper
