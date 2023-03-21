IMG ?= yangpg9/static-k8s-cloud-manager

.PHONY: docker-build deploy
docker-build:
	docker buildx rm static-k8s-cloud-manager || true
	docker buildx create --use --driver docker-container \
		   --name static-k8s-cloud-manager
	docker buildx build \
		--platform linux/amd64,linux/arm64 -t $(IMG) \
		--build-arg http_proxy=$(http_proxy) \
		--build-arg https_proxy=$(https_proxy) \
		--build-arg no_proxy=$(no_proxy) \
		--push .
	docker buildx rm static-k8s-cloud-manager

deploy:
	kustomize build kustomization/base | kubectl apply -f -