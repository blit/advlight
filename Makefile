default: help

help:
	@echo 'make build to build'
	@echo 'make deploy to build and deploy'

build:
	go-bindata -o views/assets/bindata.go -pkg assets wwwroot/...
	CGO_ENABLED=0 GOOS=linux go build advlight.go
  
deploy: build
  cp advlight chcclights
	rsync -avz -e ssh chcclights bcatickets.blit.com:
	rm chcclights
	ssh -C bcatickets.blit.com "sudo systemctl restart chcclights.service"

	
