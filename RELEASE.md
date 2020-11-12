# How software release works

## Official docker image
Official release is built by docker hub. It is triggered by any tag matches `/^[0-9.]+$/` or `/^v[0-9.]+$/` pushed to GitHub origin.