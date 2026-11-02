FROM golang:1.26

WORKDIR /workspace/backend
COPY backend/go.mod ./
COPY backend ./

EXPOSE 8080
CMD ["go", "run", "./cmd/server"]
