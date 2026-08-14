FROM node:22-alpine AS web-build
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-build /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -o /out/hr-server ./cmd/hr-server

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=go-build /out/hr-server /hr-server
COPY --from=go-build /src/contracts ./contracts
COPY --from=go-build /src/policies ./policies
COPY --from=go-build /src/departments ./departments
COPY --from=go-build /src/docs ./docs
ENV HR_SERVER_ROOT=/app
ENV HR_SERVER_LISTEN=:7777
EXPOSE 7777
ENTRYPOINT ["/hr-server"]
