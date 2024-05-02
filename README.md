# Benchmark

Benchmark is a tool to measure the performance of image pulling in a Kubernetes cluster. It aims to provider a standardized and reproducible method of measuring performance to enable easy comparison. The benchmark aims to replicate realistic image pulling behavior rather than measuring peak performance of individual image pulling. This is achieved by deploying daemonsets and allowing all of the pods to deploy. The measurements are fetched through the pod events which are reported by the CRI. Performing the measurements in this manor is critical for testing different caching solutions which may not work optimally the first time and image is pulled.

A critical part of achieving reproducible benchmarks are static images with known sizes and layer counts. Image sizes and layer count has a large impact on the image pull duration which means that it is important to measure for all different types. Static [benchmark images](https://github.com/spegel-org/benchmark/pkgs/container/benchmark) have for this reason been produced to enable measurements to be compared with each other. The images are produced twice, a v1 and a v2. The two different versions enables a simulated upgrade from one version to the other.

## How To

Prerequisites for running the benchmarks.

* [Latest version](https://github.com/spegel-org/benchmark/releases/latest) of the benchmark tool
* Access to a Kubernetes cluster

Run the benchmark measurements for the specified images.

```bash
benchmark measure --result-dir $RESULT_DIR --kubeconfig $KUBECONFIG --namespace spegel-benchmark --images ghcr.io/spegel-org/benchmark:v1-10MB-1 ghcr.io/spegel-org/benchmark:v2-10MB-1
```

Generate graphs for the measurements to visualize the results.

```bash
benchmark analyze --path $RESULT
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
