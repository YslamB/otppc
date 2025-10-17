
deploy:
	@echo "Started building..."
	@GOOS=linux GOARCH=amd64 go build -o ./bin/otp .
	@echo "Building done"

	@echo "Stopping remote service..."
	@ssh ubuntu@your_ip_address "sudo -S systemctl stop otp.service"

	@echo "Deploying..."
	@scp ./bin/otp ubuntu@your_ip_address:/var/www/otp
	# @scp ./.env ubuntu@your_ip_address:/var/www/otp
	
	@echo "Starting remote service..."
	@ssh ubuntu@your_ip_address "sudo -S systemctl start otp.service"
	@echo "Done"

