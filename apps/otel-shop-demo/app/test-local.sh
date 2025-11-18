#!/bin/bash
# Local testing script for otel-shop-demo

echo "=== Testing Backend API ==="
echo "Getting all products:"
curl -s http://localhost:8080/api/products | python3 -m json.tool | head -30

echo -e "\n\n=== Testing single product ==="
curl -s http://localhost:8080/api/products/1 | python3 -m json.tool

echo -e "\n\n=== Testing product search ==="
curl -s "http://localhost:8080/api/products/search?q=laptop" | python3 -m json.tool

echo -e "\n\n=== Frontend URL ==="
echo "Frontend is available at: http://localhost:3000"

echo -e "\n=== Container Status ==="
podman ps --pod --filter pod=otel-shop-pod

echo -e "\n=== Pod Status ==="
podman pod ps
