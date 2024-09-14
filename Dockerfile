# syntax=docker/dockerfile:1
FROM golang:1.22

# Add image label
LABEL image="ds-codegen"

# Set destination for COPY
WORKDIR /app

COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux ./build.sh

# Run
CMD ["./Codegen"]
