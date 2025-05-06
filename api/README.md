# API
This is where all the api related code is stored. The api folder contains the following subfolders:
- **google**: This folder contains the Google API files required for generating a gRPC gateway endpoint for an API.
- **tracer**: This folder contains the proto files for the tracer service. The tracer service is responsible for receiving trace data from the OpenTelemetry collector and pushing it to the Kafka queue.