package measure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
)

type Result struct {
	Name       string
	Benchmarks []Benchmark
}

type Benchmark struct {
	Image        string
	Measurements []Measurement
}

type Measurement struct {
	Start    time.Time
	Stop     time.Time
	Duration time.Duration
}

func Measure(ctx context.Context, kubeconfigPath, namespace, identifier, resultDir string, images []string) error {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}
	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}

	ts := time.Now().Unix()
	runName := fmt.Sprintf("spegel-benchmark-%d", ts)
	_, err = cs.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if errors.IsNotFound(err) {
		ns := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		_, err := cs.CoreV1().Namespaces().Create(ctx, &ns, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	err = clearImages(ctx, cs, dc, namespace, images)
	if err != nil {
		return err
	}
	defer func() {
		//nolint:errcheck // ignore
		cs.AppsV1().DaemonSets(namespace).Delete(ctx, runName, metav1.DeleteOptions{})
	}()
	result := Result{
		Name: identifier,
	}
	for _, image := range images {
		bench, err := measureImagePull(ctx, cs, dc, namespace, runName, image)
		if err != nil {
			return err
		}
		result.Benchmarks = append(result.Benchmarks, bench)
	}
	err = clearImages(ctx, cs, dc, namespace, images)
	if err != nil {
		return err
	}

	fileName := fmt.Sprintf("%s.json", identifier)
	file, err := os.Create(path.Join(resultDir, fileName))
	if err != nil {
		return err
	}
	defer file.Close()
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	_, err = file.Write(b)
	if err != nil {
		return err
	}

	return nil
}

func clearImages(ctx context.Context, cs kubernetes.Interface, dc dynamic.Interface, namespace string, images []string) error {
	remove := fmt.Sprintf("crictl rmi %s || true", strings.Join(images, " "))
	commands := []string{"/bin/sh", "-c", fmt.Sprintf("chroot /host /bin/bash -c '%s'; sleep infinity;", remove)}
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spegel-clear-image",
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "spegel-clear-image"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "spegel-clear-image",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "clear",
							Image:           "docker.io/library/alpine:3.18.4@sha256:48d9183eb12a05c99bcc0bf44a003607b8e941e1d4f41f9ad12bdcc4b5672f86",
							ImagePullPolicy: "IfNotPresent",
							Command:         commands,
							Stdin:           true,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "host-root",
									MountPath: "/host",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "host-root",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/",
								},
							},
						},
					},
				},
			},
		},
	}
	_, err := cs.AppsV1().DaemonSets(namespace).Create(ctx, ds, metav1.CreateOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	defer func() {
		//nolint:errcheck // ignore
		cs.AppsV1().DaemonSets(namespace).Delete(ctx, ds.ObjectMeta.Name, metav1.DeleteOptions{})
	}()
	err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 10*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		gvr := schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "daemonsets",
		}
		u, err := dc.Resource(gvr).Namespace(namespace).Get(ctx, ds.ObjectMeta.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		res, err := status.Compute(u)
		if err != nil {
			return false, err
		}
		if res.Status != status.CurrentStatus {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return err
	}
	return nil
}

func measureImagePull(ctx context.Context, cs kubernetes.Interface, dc dynamic.Interface, namespace, name, image string) (Benchmark, error) {
	ds, err := cs.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return Benchmark{}, err
	}
	if errors.IsNotFound(err) {
		ds := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: appsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": name},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": name,
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:            "benchmark",
								Image:           image,
								ImagePullPolicy: "IfNotPresent",
								// Keep container running
								Stdin: true,
							},
						},
					},
				},
			},
		}
		_, err = cs.AppsV1().DaemonSets(namespace).Create(ctx, ds, metav1.CreateOptions{})
		if err != nil {
			return Benchmark{}, err
		}
	} else {
		ds.Spec.Template.Spec.Containers[0].Image = image
		_, err := cs.AppsV1().DaemonSets(namespace).Update(ctx, ds, metav1.UpdateOptions{})
		if err != nil {
			return Benchmark{}, err
		}
	}

	err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 30*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		gvr := schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "daemonsets",
		}
		u, err := dc.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		res, err := status.Compute(u)
		if err != nil {
			return false, err
		}
		if res.Status != status.CurrentStatus {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return Benchmark{}, err
	}

	podList, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", name)})
	if err != nil {
		return Benchmark{}, err
	}
	if len(podList.Items) == 0 {
		return Benchmark{}, fmt.Errorf("received empty benchmark pod list")
	}
	bench := Benchmark{
		Image: image,
	}
	for _, pod := range podList.Items {
		eventList, err := cs.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{FieldSelector: fmt.Sprintf("involvedObject.name=%s", pod.Name), TypeMeta: metav1.TypeMeta{Kind: "Pod"}})
		if err != nil {
			return Benchmark{}, err
		}
		pullingEvent, err := getEvent(eventList.Items, "Pulling")
		if err != nil {
			return Benchmark{}, err
		}
		pulledEvent, err := getEvent(eventList.Items, "Pulled")
		if err != nil {
			return Benchmark{}, err
		}
		d, err := parsePullMessage(pulledEvent.Message)
		if err != nil {
			return Benchmark{}, err
		}
		bench.Measurements = append(bench.Measurements, Measurement{Start: pullingEvent.FirstTimestamp.Time, Stop: pullingEvent.FirstTimestamp.Time.Add(d), Duration: d})
	}
	return bench, nil
}

func getEvent(events []corev1.Event, reason string) (corev1.Event, error) {
	for _, event := range events {
		if event.Reason != reason {
			continue
		}
		return event, nil
	}
	return corev1.Event{}, fmt.Errorf("could not find event with reason %s", reason)
}

func parsePullMessage(msg string) (time.Duration, error) {
	r, err := regexp.Compile(`\" in (.*) \(`)
	if err != nil {
		return 0, err
	}
	match := r.FindStringSubmatch(msg)
	if len(match) < 2 {
		return 0, fmt.Errorf("could not find image pull duration")
	}
	s := match[1]
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	return d, nil
}
