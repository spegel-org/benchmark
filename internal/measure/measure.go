package measure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
)

type Result struct {
	Benchmarks []Benchmark `json:"benchmarks"`
}

type Benchmark struct {
	Image        string        `json:"image"`
	Measurements []Measurement `json:"measurements"`
}

type Measurement struct {
	Start    time.Time     `json:"start"`
	Stop     time.Time     `json:"stop"`
	Duration time.Duration `json:"duration"`
}

func Measure(ctx context.Context, kubeconfigPath, namespace, resultDir string, images []string) error {
	log := logr.FromContextOrDiscard(ctx)

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

	// Create namespace for benchmark.
	_, err = cs.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	if kerrors.IsNotFound(err) {
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

	// Run image pull measurements.
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := cs.AppsV1().DaemonSets(namespace).Delete(ctx, runName, metav1.DeleteOptions{})
		if err != nil {
			log.Error(err, "could not delete measure daemonset")
		}
	}()
	result := Result{}
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

	// Write measurement results.
	fileName := fmt.Sprintf("benchmark-%d.json", time.Now().Unix())
	file, err := os.Create(filepath.Join(resultDir, fileName))
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
	log := logr.FromContextOrDiscard(ctx)
	log.Info("clearing images")

	removeImages := fmt.Sprintf("crictl rmi %s || true", strings.Join(images, " "))
	script := fmt.Sprintf(`#!/bin/sh
chroot /host /bin/bash -c '%s'
echo "Cleanup run"
sleep infinity &
wait $!`, removeImages)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spegel-clear-image",
		},
		Data: map[string]string{
			"run.sh": script,
		},
	}
	_, err := cs.CoreV1().ConfigMaps(namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	filePerm := int32(0o755)
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
							Image:           "docker.io/library/alpine:3.21.3@sha256:a8560b36e8b8210634f77d9f7f9efd7ffa463e380b75e2e74aff4511df3ef88c",
							ImagePullPolicy: "IfNotPresent",
							Command:         []string{"/scripts/run.sh"},
							Stdin:           true,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "scripts",
									MountPath: "/scripts/run.sh",
									SubPath:   "run.sh",
								},
								{
									Name:      "host-root",
									MountPath: "/host",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "scripts",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "spegel-clear-image",
									},
									DefaultMode: &filePerm,
								},
							},
						},
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
	_, err = cs.AppsV1().DaemonSets(namespace).Create(ctx, ds, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := cs.AppsV1().DaemonSets(namespace).Delete(ctx, ds.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Error(err, "could not delete image cleanup daemonset")
		}
		err = cs.CoreV1().ConfigMaps(namespace).Delete(ctx, cm.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Error(err, "could not delete image cleanup config map")
		}
	}()

	err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 10*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		gvr := schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "daemonsets",
		}
		u, err := dc.Resource(gvr).Namespace(namespace).Get(ctx, ds.Name, metav1.GetOptions{})
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
	log := logr.FromContextOrDiscard(ctx).WithValues("image", image)
	log.Info("measuring pull performance")
	ds, err := cs.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return Benchmark{}, err
	}
	if kerrors.IsNotFound(err) {
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

	log.Info("waiting for rollout completion")
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
			log.Info("waiting for rollout", "message", res.Message)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return Benchmark{}, err
	}

	log.Info("collecting image pull durations")
	podList, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", name)})
	if err != nil {
		return Benchmark{}, err
	}
	if len(podList.Items) == 0 {
		return Benchmark{}, errors.New("received empty benchmark pod list")
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
		bench.Measurements = append(bench.Measurements, Measurement{Start: pullingEvent.FirstTimestamp.Time, Stop: pullingEvent.FirstTimestamp.Add(d), Duration: d})
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
	//nolint: gocritic // We should never panic and always check errors.
	r, err := regexp.Compile(`\" in (.*) \(`)
	if err != nil {
		return 0, err
	}
	match := r.FindStringSubmatch(msg)
	if len(match) < 2 {
		return 0, errors.New("could not find image pull duration")
	}
	s := match[1]
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	return d, nil
}
