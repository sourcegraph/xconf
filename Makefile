.PHONY: docker-image push-docker-image

docker-image:
	docker build -t sqs1/xconf .

push-docker-image: docker-image
	docker push sqs1/xconf
