package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// service = plugin name, module = specific module in the plugin, status = alarm status like "up" or "down"
//
// service, module name and status can be nil if not applicable
//
// instead of directly giving them as string, giving them as pointer to string
func SendZulipAlarm(message string, service string, module string, status string) error {
	var lastAlarm ZulipAlarm
	var lastAlarms []ZulipAlarm
	var err error

	lastAlarm, err = GetLastZulipAlarm(service, module)
	lastAlarms, err = GetLastZulipAlarms(service, module)

	if err != nil {
		Logger.Error().Err(err).Msg("Failed to get last alarm from database")
		return err
	}

	// no extra checks needed if there are no previous alarms
	if len(lastAlarms) == 0 {
		var sendErr error

		if !IsTestMode() {
			sendErr = sendZulipAlarm(message)
		} else {
			sendErr = nil
		}

		if sendErr == nil {
			DB.Create(&ZulipAlarm{
				ProjectIdentifier: GlobalConfig.ProjectIdentifier,
				Hostname:          GlobalConfig.Hostname,
				Content:           message,
				Service:           service,
				Module:            module,
				Status:            status,
			})
		}

		return sendErr
	}

	// checking if alarms are duplicate
	firstStatus := lastAlarms[0].Status
	allSame := true
	for _, alarm := range lastAlarms {
		if alarm.Status != firstStatus {
			allSame = false
			break
		}
	}

	// checks if last alarms have the same status as the new one and if the limit is reached
	// if so, skip sending the alarm
	if allSame && firstStatus == status && len(lastAlarms) >= GlobalConfig.ZulipAlarm.Limit {
		var message string
		message = fmt.Sprintf("Zulip alarm limit (%d) has reached to limit for %s, skipping this one", GlobalConfig.ZulipAlarm.Limit, service)
		Logger.Info().Str("service", service).Msg(message)
		return fmt.Errorf(message)
	}

	if !IsTestMode() {
		if time.Since(lastAlarm.CreatedAt) < time.Duration(GlobalConfig.ZulipAlarm.Interval)*time.Minute {
			var message string
			message = fmt.Sprintf("Enough time is not passed since the last alarm from %s, skipping this one", service)
			Logger.Info().Str("service", service).Msg(message)
			return fmt.Errorf(message)
		}
	}

	var sendErr error
	if !IsTestMode() {
		sendErr = sendZulipAlarm(message)
	} else {
		Logger.Info().Msg("Test mode is enabled, skipping sending Zulip alarm")
		sendErr = nil
	}

	if sendErr == nil {
		DB.Create(&ZulipAlarm{
			ProjectIdentifier: GlobalConfig.ProjectIdentifier,
			Hostname:          GlobalConfig.Hostname,
			Content:           message,
			Service:           service,
			Module:            module,
			Status:            status,
		})
	}

	return sendErr
}

func GetLastZulipAlarm(service string, module string) (ZulipAlarm, error) {
	var lastAlarm ZulipAlarm

	err := DB.Where("project_identifier = ? AND hostname = ? AND service = ? AND module = ?",
		GlobalConfig.ProjectIdentifier,
		GlobalConfig.Hostname,
		service,
		module).Order("id DESC").Limit(1).Find(&lastAlarm).Error

	if err != nil {
		Logger.Error().Err(err).Msg("Failed to get last alarm from database")
		return lastAlarm, err
	}

	return lastAlarm, err
}

func GetLastZulipAlarms(service string, module string) ([]ZulipAlarm, error) {
	var lastAlarms []ZulipAlarm

	err := DB.Where("project_identifier = ? AND hostname = ? AND service = ? AND module = ?",
		GlobalConfig.ProjectIdentifier,
		GlobalConfig.Hostname,
		service,
		module).
		Order("id DESC").
		Limit(GlobalConfig.ZulipAlarm.Limit).
		Find(&lastAlarms).Error

	return lastAlarms, err
}

func sendZulipAlarm(message string) error {
	if !GlobalConfig.ZulipAlarm.Enabled {
		Logger.Warn().Msg("Zulip alarm is bot enabled in the configuration")
		return fmt.Errorf("zulip alarm not enabled")
	}

	if GlobalConfig.ZulipAlarm.BotApi.Enabled {
		if IsTestMode() {
			Logger.Info().Msg("Test mode is enabled, skipping sending Zulip Bot API alarm")
			return nil
		}

		err := sendZulipBotApiAlarm(message)
		if err != nil {
			Logger.Error().Err(err).Msgf("failed to send Zulip Bot API alarm")
			return err
		}
	} else {
		for _, url := range GlobalConfig.ZulipAlarm.WebhookUrls {
			err := sendZulipWebhookAlarm(url, message)
			if err != nil {
				Logger.Error().Err(err).Msgf("Failed to send Zulip webhook alarm to %s", url)
				return err
			}
		}
	}

	return nil
}

// Placeholder for future implementation
func sendZulipBotApiAlarm(message string) error {

	return nil
}

func sendZulipWebhookAlarm(url string, message string) error {
	payload := map[string]string{
		"text": message,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to marshal Zulip webhook payload")
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		Logger.Error().Err(err).Msg("failed to send request")
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		Logger.Error().Msgf("Received non-2xx response: %d", resp.StatusCode)
		return fmt.Errorf("zulip webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func CreateRedmineNews(news News) error {
	if IsTestMode() {
		Logger.Info().Msg("Test mode is enabled, skipping creating Redmine news")

		err := DB.Create(&news).Error
		if err != nil {
			Logger.Error().Err(err).Int("id", int(news.Id)).Msg("Failed to store news in local database")
			return err
		}
	}

	if !IsTestMode() {
		if !GlobalConfig.Redmine.Enabled {
			Logger.Warn().Msg("Redmine integration is not enabled in the configuration")
			return nil
		}

		if GlobalConfig.Redmine.ApiKey == "" || GlobalConfig.Redmine.Url == "" {
			Logger.Error().Msg("Redmine API key or URL not configured")
			return fmt.Errorf("Redmine API key or URL not configured")
		}

		newsUrl := fmt.Sprintf("%s/projects/%s/news.json", GlobalConfig.Redmine.Url, GlobalConfig.ProjectIdentifier)

		redmineNews := RedmineNews{
			News: news,
		}

		jsonBody, err := json.Marshal(redmineNews)
		if err != nil {
			Logger.Error().Err(err).Msg("Failed to marshal news creation request")
			return err
		}

		req, err := http.NewRequest("POST", newsUrl, bytes.NewBuffer(jsonBody))
		if err != nil {
			Logger.Error().Err(err).Str("url", newsUrl).Msg("Failed to create news request")
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Redmine-API-Key", GlobalConfig.Redmine.ApiKey)
		req.Header.Set("User-Agent", "Monokit/devel")

		client := &http.Client{Timeout: time.Second * 30}
		resp, err := client.Do(req)
		if err != nil {
			Logger.Error().Err(err).Str("url", newsUrl).Msg("Failed to send news request")
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			Logger.Error().Int("status_code", resp.StatusCode).Str("response", string(body)).Msg("Failed to send Redmine news")
			return fmt.Errorf("failed to send Redmine news, status code: %d", resp.StatusCode)
		}

		err = DB.Create(&news).Error
		if err != nil {
			Logger.Error().Err(err).Int("id", int(news.Id)).Msg("Failed to store news in local database")
			return err
		}
	}

	return nil
}

func GetLastRedmineIssue(service string, module string) (Issue, error) {
	var lastIssue Issue

	err := DB.
		Where("project_identifier = ? AND hostname = ? AND service = ? AND module = ?",
			GlobalConfig.ProjectIdentifier,
			GlobalConfig.Hostname,
			service,
			module).
		Order("table_id DESC").
		Limit(1).
		Find(&lastIssue).Error

	if err != nil {
		Logger.Error().Err(err).Msg("Failed to get last issue from database")
		return lastIssue, err
	}

	return lastIssue, nil
}

func GetLastRedmineIssues(service string, module string) ([]Issue, error) {
	var lastIssues []Issue

	err := DB.
		Where("project_identifier = ? AND hostname = ? AND service = ? AND module = ?",
			GlobalConfig.ProjectIdentifier,
			GlobalConfig.Hostname,
			service,
			module).
		Order("table_id DESC").
		Limit(GlobalConfig.Redmine.Limit).
		Find(&lastIssues).Error

	if err != nil {
		Logger.Error().Err(err).Msg("Failed to get last issues from database")
		return lastIssues, err
	}

	return lastIssues, nil
}

// issue.Service = plugin name, issue.Module = specific module in the plugin, issue.Status = alarm status like "up" or "down"
//
// service, module name and status can be nil if not applicable
//
// instead of directly giving them as string, giving them as pointer to string
//
// Example input:
//
//	issue := lib.Issue{
//		ProjectIdentifier: lib.GlobalConfig.ProjectIdentifier,
//		Hostname:          lib.GlobalConfig.Hostname,
//		Subject:           fmt.Sprintf("%s için sistem yükü %.2f üstüne çıktı", lib.GlobalConfig.Hostname, loadLimit),
//		Description:       alarmMessage,
//		Service:           &pluginName,
//		Module:            &moduleName,
//		Status:            &down
//	}
func CreateRedmineIssue(issue Issue) error {
	if !IsTestMode() {
		if !GlobalConfig.Redmine.Enabled {
			Logger.Warn().Msg("Redmine integration is not enabled in the configuration")
			return nil
		}

		if GlobalConfig.Redmine.ApiKey == "" || GlobalConfig.Redmine.Url == "" {
			Logger.Error().Msg("Redmine API key or URL not configured")
			return fmt.Errorf("Redmine API key or URL not configured")
		}
	}

	issue.ProjectIdentifier = GlobalConfig.ProjectIdentifier
	issue.Hostname = GlobalConfig.Hostname

	var lastIssue Issue
	var lastIssues []Issue
	var err error

	if issue.Service != "" {
		lastIssue, err = GetLastRedmineIssue(issue.Service, issue.Module)
		lastIssues, err = GetLastRedmineIssues(issue.Service, issue.Module)
	}

	if issue.Service == "" {
		err = DB.
			Where("project_identifier = ? AND hostname = ?",
				GlobalConfig.ProjectIdentifier,
				GlobalConfig.Hostname).
			Order("table_id DESC").
			Limit(1).
			Find(&lastIssue).Error

		err = DB.
			Where("project_identifier = ? AND hostname = ?",
				GlobalConfig.ProjectIdentifier,
				GlobalConfig.Hostname).
			Order("table_id DESC").
			Limit(GlobalConfig.Redmine.Limit).
			Find(&lastIssues).Error
	}

	if err != nil {
		Logger.Error().Err(err).Msg("Failed to get last issues from database")
		return err
	}

	// no extra checks needed if there are no previous issues
	if len(lastIssues) == 0 {
		Logger.Info().Str("subject", issue.Subject).Msg("Creating new Redmine issue")
		return createNewRedmineIssue(issue)
	}

	// checking if issues are duplicate
	firstStatus := lastIssues[0].Status
	allSame := true
	for _, alarm := range lastIssues {
		if alarm.Status != firstStatus {
			allSame = false
			break
		}
	}

	Logger.Debug().Bool("all_same", allSame).Str("first_status", fmt.Sprintf("%v", firstStatus)).Str("new_status", fmt.Sprintf("%v", issue.Status)).Int("last_issues_count", len(lastIssues)).Msg("Redmine issue duplication check")

	// checks if last issues have the same status as the new one and if the limit is reached
	// if so, skip sending the issue
	if allSame && firstStatus == issue.Status && len(lastIssues) >= GlobalConfig.Redmine.Limit {
		var message string
		if issue.Service != "" {
			message = fmt.Sprintf("Redmine issue limit (%d) has reached to limit for %s, skipping this one", GlobalConfig.Redmine.Limit, issue.Service)
			Logger.Info().Str("service", issue.Service).Msg(message)
		} else {
			message = fmt.Sprintf("Redmine issue limit (%d) has reached to limit, skipping this one", GlobalConfig.Redmine.Limit)
			Logger.Info().Msg(message)
		}
		return fmt.Errorf(message)
	}

	if !IsTestMode() {
		if time.Since(lastIssue.CreatedAt) < time.Duration(GlobalConfig.Redmine.Interval)*time.Minute {
			var message string
			if issue.Service != "" {
				message = fmt.Sprintf("Enough time is not passed since the last Issue from %s, skipping this one", issue.Service)
				Logger.Info().Str("service", issue.Service).Msg(message)
			} else {
				message = "Enough time is not passed since the last issue, skipping this one"
				Logger.Info().Msg(message)
			}
			return fmt.Errorf(message)
		}
	}

	if lastIssue.Status == issue.Status {
		// same status but has notes: add note to existing issue
		if issue.Notes != "" {
			existingIssue := findRecentSimilarIssue(issue.Service, issue.Module, 6)
			if existingIssue != nil {
				Logger.Info().Int("existing_issue_id", existingIssue.Id).Str("subject", issue.Subject).Msg("Adding note to open issue (same status)")
				issue.Id = existingIssue.Id
				return updateRedmineIssueStatus(existingIssue.Id, issue)
			}
		}
		Logger.Info().Str("subject", issue.Subject).Msg("Last issue status same as new issue, skipping creating new one")
		return nil
	}

	// if the new issue has a different status than the last issue, update the status of the last issue
	if lastIssue.Status != issue.Status {
		existingIssue := findRecentSimilarIssue(issue.Service, issue.Module, 6)
		if existingIssue != nil {
			Logger.Info().Int("existing_issue_id", existingIssue.Id).Str("subject", issue.Subject).Msg("Found existing issue, updating status instead of creating new one")
			issue.Id = existingIssue.Id
			return updateRedmineIssueStatus(existingIssue.Id, issue)
		}
	}

	// if the new issue is "down" type and the last issue is "up" type too, then instead of creating a new issue, reopen the existing one
	// find if there is an existing issue with the same subject in the last 6 hours
	if issue.Status == "down" && lastIssue.Status == "up" {
		existingIssue := findRecentSimilarIssue(issue.Service, issue.Module, 6)
		if existingIssue != nil {
			Logger.Info().Int("existing_issue_id", existingIssue.Id).Str("subject", issue.Subject).Msg("Found existing issue, reopening instead of creating new one")
			issue.Id = existingIssue.Id
			return reopenRedmineIssue(existingIssue.Id, issue)
		}
	}

	Logger.Info().Str("subject", issue.Subject).Msg("Creating new Redmine issue")
	return createNewRedmineIssue(issue)
}

func findRecentSimilarIssue(serviceName string, moduleName string, hoursBack int) *Issue {
	now := time.Now()
	hoursAgo := now.Add(-time.Duration(hoursBack) * time.Hour)

	var existingIssues []Issue
	result := DB.Where("service = ? AND module = ? AND created_at > ?", serviceName, moduleName, hoursAgo).Order("id DESC").Find(&existingIssues)
	if result.Error != nil {
		Logger.Error().Err(result.Error).Msg("Failed to query database for existing issues")
		return nil
	}

	if len(existingIssues) > 0 {
		Logger.Debug().Int("count", len(existingIssues)).Msg("Found existing issues in database")
		return &existingIssues[0]
	}

	return nil
}

func updateRedmineIssueStatus(issueId int, issue Issue) error {
	if IsTestMode() {
		Logger.Info().Msg("Test mode is enabled, skipping updating Redmine issue status")

		issueId = issue.Id
		result := DB.Create(&issue)
		if result.Error != nil {
			Logger.Error().Err(result.Error).Int("issue_id", issue.Id).Msg("Failed to store issue in local database")
			return result.Error
		}
		return nil
	}

	if !IsTestMode() {
		issueResp, err := GetIssueFromRedmine(issueId)
		if err != nil {
			Logger.Error().Err(err).Int("issue_id", issueId).Msg("Failed to fetch issue from Redmine")
			return err
		}

		isAssigned := issueResp.Issue.AssignedTo != nil
		isClosing := issue.StatusId == IssueStatus.Closed

		Logger.Debug().
			Int("issue_id", issueId).
			Bool("is_assigned", isAssigned).
			Bool("is_closing", isClosing).
			Int("new_status_id", issue.StatusId).
			Str("notes", issue.Notes).
			Msg("Preparing to update Redmine issue")

		var updateIssue Issue

		// Always add notes if provided
		if issue.Notes != "" {
			updateIssue.Notes = issue.Notes
		}

		// Update status logic:
		// - If unassigned: always update status (can update to any status including Closed)
		// - If assigned: can update status to Urgent/Feedback/etc but CANNOT set to Closed
		//   (don't auto-close assigned issues - someone is working on it)
		shouldUpdateStatus := !isAssigned || !isClosing

		if shouldUpdateStatus {
			updateIssue.StatusId = issue.StatusId
			Logger.Debug().Int("issue_id", issueId).Int("new_status_id", issue.StatusId).Msg("Will update status")
		} else {
			Logger.Info().Int("issue_id", issueId).Msg("Skipping status update: issue is assigned and trying to close")
		}

		var updateData RedmineIssue
		updateData.Issue = updateIssue

		jsonBody, err := json.Marshal(updateData)
		if err != nil {
			Logger.Error().Err(err).Msg("Failed to marshal issue update request")
			return err
		}

		Logger.Debug().Int("issue_id", issueId).Str("json_payload", string(jsonBody)).Msg("Sending update to Redmine")

		updateUrl := fmt.Sprintf("%s/issues/%d.json", GlobalConfig.Redmine.Url, issueId)
		req, err := http.NewRequest("PUT", updateUrl, bytes.NewBuffer(jsonBody))
		if err != nil {
			Logger.Error().Err(err).Str("url", updateUrl).Msg("Failed to create issue update request")
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Redmine-API-Key", GlobalConfig.Redmine.ApiKey)
		req.Header.Set("User-Agent", "Monokit/devel")

		postClient := &http.Client{Timeout: time.Second * 30}
		resp, err := postClient.Do(req)
		if err != nil {
			Logger.Error().Err(err).Str("url", updateUrl).Msg("Failed to send issue update request")
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			Logger.Error().Int("status_code", resp.StatusCode).Str("response", string(body)).Msg("Failed to update Redmine issue")
			return fmt.Errorf("failed to update issue, status code: %d", resp.StatusCode)
		}

		Logger.Info().Int("issue_id", issueId).Int("status_code", resp.StatusCode).Msg("Successfully updated Redmine issue via API")

		result := DB.Create(&issue)
		if result.Error != nil {
			Logger.Error().Err(result.Error).Int("issue_id", issue.Id).Msg("Failed to store issue in local database")
		}
	}

	Logger.Info().Int("issue_id", issueId).Int("status_id", issue.StatusId).Str("status", issue.Status).Msg("Successfully updated Redmine issue status")
	return nil
}

func reopenRedmineIssue(issueId int, issue Issue) error {
	updateData := map[string]interface{}{
		"issue": map[string]interface{}{
			"status_id": IssueStatus.Feedback,
		},
	}

	if IsTestMode() {
		result := DB.Create(&issue)
		if result.Error != nil {
			Logger.Error().Err(result.Error).Int("issue_id", issue.Id).Msg("Failed to store issue in local database")
		}
	}

	if !IsTestMode() {
		jsonBody, err := json.Marshal(updateData)
		if err != nil {
			Logger.Error().Err(err).Msg("Failed to marshal issue update request")
			return err
		}

		updateUrl := fmt.Sprintf("%s/issues/%d.json", GlobalConfig.Redmine.Url, issueId)
		req, err := http.NewRequest("PUT", updateUrl, bytes.NewBuffer(jsonBody))
		if err != nil {
			Logger.Error().Err(err).Str("url", updateUrl).Msg("Failed to create issue update request")
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Redmine-API-Key", GlobalConfig.Redmine.ApiKey)
		req.Header.Set("User-Agent", "Monokit/devel")

		client := &http.Client{Timeout: time.Second * 30}
		resp, err := client.Do(req)
		if err != nil {
			Logger.Error().Err(err).Str("url", updateUrl).Msg("Failed to send issue update request")
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			Logger.Error().Int("status_code", resp.StatusCode).Str("response", string(body)).Msg("Failed to update Redmine issue")
			return fmt.Errorf("failed to update issue, status code: %d", resp.StatusCode)
		}

		result := DB.Create(&issue)
		if result.Error != nil {
			Logger.Error().Err(result.Error).Int("issue_id", issue.Id).Msg("Failed to store issue in local database")
		}
	}

	Logger.Info().Int("issue_id", issueId).Msg("Successfully reopened Redmine issue")
	return nil
}

func createNewRedmineIssue(issue Issue) error {
	var createIssue Issue
	createIssue.ProjectId = GlobalConfig.ProjectIdentifier
	createIssue.TrackerId = issue.TrackerId
	createIssue.Subject = issue.Subject
	createIssue.Description = issue.Description
	createIssue.PriorityId = issue.PriorityId
	createIssue.StatusId = issue.StatusId

	var createData RedmineIssue
	createData.Issue = createIssue

	if IsTestMode() {
		Logger.Info().Msg("Test mode is enabled, skipping creating Redmine issue")

		var lastIssueId int

		err := DB.Model(&Issue{}).Select("MAX(id)").Row().Scan(&lastIssueId)
		if err != nil {
			lastIssueId = 1
		}

		issue.Id = lastIssueId + 1
		result := DB.Create(&issue)
		if result.Error != nil {
			Logger.Error().Err(result.Error).Int("issue_id", issue.Id).Msg("Failed to store issue in local database")
		}
	}

	if !IsTestMode() {
		jsonBody, err := json.Marshal(createData)
		if err != nil {
			Logger.Error().Err(err).Msg("Failed to marshal issue creation request")
			return err
		}

		createUrl := GlobalConfig.Redmine.Url + "/issues.json"
		req, err := http.NewRequest("POST", createUrl, bytes.NewBuffer(jsonBody))
		if err != nil {
			Logger.Error().Err(err).Str("url", createUrl).Msg("Failed to create issue creation request")
			fmt.Println(err)
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Redmine-API-Key", GlobalConfig.Redmine.ApiKey)
		req.Header.Set("User-Agent", "Monokit/devel")

		client := &http.Client{Timeout: time.Second * 30}
		resp, err := client.Do(req)
		if err != nil {
			Logger.Error().Err(err).Str("url", createUrl).Msg("Failed to send issue creation request")
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			Logger.Error().Int("status_code", resp.StatusCode).Str("response", string(body)).Msg("Failed to create Redmine issue")
			return fmt.Errorf("failed to create issue, status code: %d", resp.StatusCode)
		}

		var response struct {
			Issue struct {
				Id int `json:"id"`
			} `json:"issue"`
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			Logger.Error().Err(err).Msg("Failed to read response body")
			return err
		}

		err = json.Unmarshal(body, &response)
		if err != nil {
			Logger.Error().Err(err).Msg("Failed to parse issue creation response")
			return err
		}

		issue.Id = response.Issue.Id
		result := DB.Create(&issue)
		if result.Error != nil {
			Logger.Error().Err(result.Error).Int("issue_id", issue.Id).Msg("Failed to store issue in local database")
		}
	}

	Logger.Info().Int("issue_id", issue.Id).Str("subject", issue.Subject).Msg("Successfully created new Redmine issue")
	return nil
}
