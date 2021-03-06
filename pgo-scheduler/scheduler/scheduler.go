package scheduler

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/crunchydata/postgres-operator/apiserver"
	"github.com/crunchydata/postgres-operator/kubeapi"

	cv2 "gopkg.in/robfig/cron.v2"
	"k8s.io/client-go/kubernetes"
)

var PolicyJobTemplate *template.Template

func Init() error {
	buf, err := ioutil.ReadFile("/pgo-config/pgo.sqlrunner-template.json")
	if err != nil {
		return err
	}
	PolicyJobTemplate = template.Must(template.New("policy").Parse(string(buf)))
	return nil
}

func New(label, namespace string, client *kubernetes.Clientset) *Scheduler {
	apiserver.ConnectToKube()
	restClient = apiserver.RESTClient
	kubeClient = client
	cronClient := cv2.New()
	var p phony
	cronClient.AddJob("* * * * *", p)

	return &Scheduler{
		namespace:  namespace,
		label:      label,
		CronClient: cronClient,
		entries:    make(map[string]cv2.EntryID),
	}
}

func (s *Scheduler) AddSchedules() error {
	configs, _ := kubeapi.ListConfigMap(kubeClient, s.label, s.namespace)

	for _, config := range configs.Items {
		if _, ok := s.entries[string(config.Name)]; ok {
			continue
		}

		contextErr := log.WithFields(log.Fields{
			"configMap": config.Name,
		})

		if len(config.Data) != 1 {
			contextErr.WithFields(log.Fields{
				"error": errors.New("Schedule configmaps should contain only one schedule"),
			}).Error("Failed reading configMap")
		}

		var schedule ScheduleTemplate
		for _, data := range config.Data {
			if err := json.Unmarshal([]byte(data), &schedule); err != nil {
				contextErr.WithFields(log.Fields{
					"error": err,
				}).Error("Failed unmarshaling configMap")
				continue
			}
		}

		if err := validate(schedule); err != nil {
			contextErr.WithFields(log.Fields{
				"error": err,
			}).Error("Failed to validate schedule")
			continue
		}

		id, err := s.schedule(schedule)
		if err != nil {
			contextErr.WithFields(log.Fields{
				"error": err,
			}).Error("Failed to schedule configMap")
			continue
		}

		log.WithFields(log.Fields{
			"configMap":  string(config.Name),
			"type":       schedule.Type,
			"schedule":   schedule.Schedule,
			"namespace":  schedule.Namespace,
			"deployment": schedule.Deployment,
			"label":      schedule.Label,
			"container":  schedule.Container,
		}).Info("Added new schedule")
		s.entries[string(config.Name)] = id
	}

	return nil
}

func (s *Scheduler) DeleteSchedules() error {
	configs, _ := kubeapi.ListConfigMap(kubeClient, s.label, s.namespace)
	for name := range s.entries {
		found := false
		for _, config := range configs.Items {
			if name == string(config.Name) {
				found = true
			}
		}

		if !found {
			log.WithFields(log.Fields{
				"scheduleName": name,
			}).Info("Removed schedule")
			s.CronClient.Remove(s.entries[name])
			delete(s.entries, name)
		}
	}
	return nil
}

func (s *Scheduler) schedule(st ScheduleTemplate) (cv2.EntryID, error) {
	var job cv2.Job

	switch st.Type {
	case "pgbackrest":
		job = st.NewBackRestSchedule()
	case "pgbasebackup":
		job = st.NewBaseBackupSchedule()
	case "policy":
		job = st.NewPolicySchedule()
	default:
		var id cv2.EntryID
		return id, fmt.Errorf("schedule type not implemented yet")
	}
	return s.CronClient.AddJob(st.Schedule, job)
}

type phony string

func (p phony) Run() {
	// This is a phony job that register with the cron service
	// that does nothing to prevent a bug that runs newly scheduled
	// jobs multiple times.
	_ = time.Now()
}
