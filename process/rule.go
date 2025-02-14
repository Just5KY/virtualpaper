/*
 * Virtualpaper is a service to manage users paper documents in virtual format.
 * Copyright (C) 2021  Tero Vierimaa
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package process

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"tryffel.net/go/virtualpaper/errors"
	"tryffel.net/go/virtualpaper/models"
)

type DocumentRule struct {
	Rule     *models.Rule
	Document *models.Document
	date     time.Time
}

type RuleTestResult struct {
	Conditions []struct {
		ConditionId int  `json:"condition_id"`
		Matched     bool `json:"matched"`
	} `json:"conditions"`
	RuleId    int    `json:"rule_id"`
	Match     bool   `json:"matched"`
	TookMs    int    `json:"took_ms"`
	Log       string `json:"log"`
	Error     string `json:"error"`
	StartedAt int    `json:"started_at"`
	StoppedAt int    `json:"stopped_at"`
}

func NewDocumentRule(document *models.Document, rule *models.Rule) DocumentRule {
	return DocumentRule{
		Rule:     rule,
		Document: document,
	}
}

func (d *DocumentRule) Match() (bool, error) {
	hasMatch := false

	logrus.Debugf("match document: %s, rule: %d", d.Document.Id, d.Rule.Id)
	for i, condition := range d.Rule.Conditions {
		if !condition.Enabled {
			logrus.Debugf("rule %d - condition: %d (id:%d), %s is disabled", d.Rule.Id, condition.Id, i+1, condition.ConditionType)
			continue
		}

		logrus.Debugf("evaluate rule %d - condition: %d (id:%d), %s", d.Rule.Id, condition.Id, i+1, condition.ConditionType)
		condText := string(condition.ConditionType)
		var ok = false
		var err error
		if strings.HasPrefix(condText, "name") {
			ok, err = d.matchText(condition, d.Document.Name)
		} else if strings.HasPrefix(condText, "description") {
			ok, err = d.matchText(condition, d.Document.Description)
		} else if strings.HasPrefix(condText, "content") {
			ok, err = d.matchText(condition, d.Document.Content)
		} else if strings.HasPrefix(condText, "metadata_has_key") {
			ok = d.hasMetadataKey(condition)
		} else if strings.HasPrefix(condText, "date") {
			ok, err = d.extractDates(condition, time.Now(), nil)
		} else if strings.HasPrefix(condText, "metadata_count") {
			ok, err = d.hasMetadataCount(condition)
		} else if condition.ConditionType == models.RuleConditionMetadataHasKey {
			ok = d.hasMetadataKey(condition)
		} else if condition.ConditionType == models.RuleConditionMetadataHasKeyValue {
			ok = d.hasMetadataKeyValue(condition)
		} else {
			err := errors.ErrInternalError
			err.ErrMsg = "unknown condition type: " + condText
			return false, err
		}
		if err != nil {
			return false, fmt.Errorf("evaluate condition: %v", err)
		}

		if condition.Inverted {
			ok = !ok
		}

		if ok {
			hasMatch = true
		} else if d.Rule.Mode == models.RuleMatchAll {
			return false, nil
		}

		if hasMatch && d.Rule.Mode == models.RuleMatchAny {
			// already found a match, skip rest of the conditions
			break
		}
	}
	return hasMatch, nil
}

type formatter struct{}

func (f *formatter) Format(entry *logrus.Entry) ([]byte, error) {

	if entry.Level == logrus.InfoLevel {
		return []byte(entry.Message + "\n"), nil
	} else {
		return []byte(fmt.Sprintf("%s - %s\n", entry.Level.String(), entry.Message)), nil
	}
}

func (d *DocumentRule) MatchTest() *RuleTestResult {
	logBuf := &bytes.Buffer{}

	logger := logrus.New()
	logger.SetOutput(logBuf)
	logger.SetFormatter(&formatter{})
	format, ok := logger.Formatter.(*logrus.TextFormatter)
	if ok {
		format.FullTimestamp = false
		format.DisableTimestamp = true

	}

	hasMatch := false

	result := &RuleTestResult{
		StartedAt: int(time.Now().UnixNano() / 1000000),
		RuleId:    d.Rule.Id,
		Conditions: []struct {
			ConditionId int  "json:\"condition_id\""
			Matched     bool "json:\"matched\""
		}{},
	}

	logger.Infof("Try to match document: %s, rule: id: %d, name: %s", d.Document.Id, d.Rule.Id, d.Rule.Name)
	for i, condition := range d.Rule.Conditions {
		if !condition.Enabled {
			logger.Warnf("rule %d - condition: %d (id:%d), %s is disabled, skipping condition", d.Rule.Id, condition.Id, i+1, condition.ConditionType)
			continue
		}

		logger.Infof("evaluate rule %d - condition: %d (id:%d), type: '%s'", d.Rule.Id, condition.Id, i+1, condition.ConditionType)
		condText := string(condition.ConditionType)
		var ok = false
		var err error
		if strings.HasPrefix(condText, "name") {
			ok, err = d.matchText(condition, d.Document.Name)
		} else if strings.HasPrefix(condText, "description") {
			ok, err = d.matchText(condition, d.Document.Description)
		} else if strings.HasPrefix(condText, "content") {
			ok, err = d.matchText(condition, d.Document.Content)
		} else if strings.HasPrefix(condText, "metadata_has_key") {
			ok = d.hasMetadataKey(condition)
		} else if strings.HasPrefix(condText, "date") {
			ok, err = d.extractDates(condition, time.Now(), logger)

			if ok {
				y, m, d := d.date.Date()
				logger.Infof("found date %d-%d-%d", y, m, d)
			}

		} else if strings.HasPrefix(condText, "metadata_count") {
			ok, err = d.hasMetadataCount(condition)
		} else if condition.ConditionType == models.RuleConditionMetadataHasKey {
			ok = d.hasMetadataKey(condition)
		} else if condition.ConditionType == models.RuleConditionMetadataHasKeyValue {
			ok = d.hasMetadataKeyValue(condition)
		} else {
			err := errors.ErrInternalError
			err.ErrMsg = "unknown condition type: " + condText
			result.Error = err.Error()
			break
		}
		if err != nil {
			e := errors.ErrInternalError
			e.ErrMsg = fmt.Errorf("evaluate condition: %v", err).Error()
			result.Error = e.Error()
			break
		}

		if condition.Inverted {
			ok = !ok
		}

		if ok {
			hasMatch = true
			logger.Infof("condition %d matched", condition.Id)
			if d.Rule.Mode == models.RuleMatchAny {
				// already found a match, skip rest of the conditions
				logger.Infof("document matches and mode is set to 'match any', skip rest conditions")
				break
			}

		} else if d.Rule.Mode == models.RuleMatchAll {
			logger.Infof("condition %d didn't match, skip rest", condition.Id)
			break
		} else {
			logger.Infof("condition %d didn't match, continuing", condition.Id)
		}
		result.Conditions = append(result.Conditions, struct {
			ConditionId int  "json:\"condition_id\""
			Matched     bool "json:\"matched\""
		}{condition.Id, ok})
	}

	result.StoppedAt = int(time.Now().UnixNano() / 1000000)
	result.TookMs = result.StoppedAt - result.StartedAt
	result.Match = hasMatch

	result.Log = logBuf.String()
	return result
}

func (d *DocumentRule) matchText(condition *models.RuleCondition, text string) (bool, error) {
	value := condition.Value
	if condition.CaseInsensitive {
		text = strings.ToLower(text)
		value = strings.ToLower(value)
	}

	switch condition.ConditionType {
	case models.RuleConditionNameIs, models.RuleConditionDescriptionIs, models.RuleConditionContentIs:
		return matchTextAllowTypo(value, text, false, true)
	case models.RuleConditionNameStarts, models.RuleConditionDescriptionStarts, models.RuleConditionContentStarts:
		return matchTextAllowTypo(value, text, true, false)
	case models.RuleConditionNameContains, models.RuleConditionDescriptionContains, models.RuleConditionContentContains:
		return matchTextAllowTypo(value, text, false, false)
	default:
		err := errors.ErrInternalError
		err.ErrMsg = fmt.Sprintf("unknown condition type: %s", condition.ConditionType)
		err.SetStack()
		return false, err
	}
}

func (d *DocumentRule) hasMetadataKey(condition *models.RuleCondition) bool {
	for _, v := range d.Document.Metadata {
		if v.KeyId == int(condition.MetadataKey) {
			return true
		}
	}
	return false
}

func (d *DocumentRule) hasMetadataKeyValue(condition *models.RuleCondition) bool {
	for _, v := range d.Document.Metadata {
		if v.KeyId == int(condition.MetadataKey) && v.ValueId == int(condition.MetadataValue) {
			return true
		}
	}
	return false
}

func (d *DocumentRule) hasMetadataCount(condition *models.RuleCondition) (bool, error) {
	limit, err := strconv.Atoi(condition.Value)
	if err != nil || limit < 0 {
		e := errors.ErrInvalid
		e.ErrMsg = "value must be a non-negative number"
		return false, e
	}

	switch condition.ConditionType {
	case models.RuleConditionMetadataCount:
		return len(d.Document.Metadata) == limit, nil
	case models.RuleConditionMetadataCountLessThan:
		return len(d.Document.Metadata) < limit, nil
	case models.RuleConditionMetadataCountMoreThan:
		return len(d.Document.Metadata) > limit, nil
	default:
		return false, fmt.Errorf("not metadata count condition: %v", condition.ConditionType)
	}
}

// Try to extract all dates from the document.
// In case there are multiple dates found, prioritice:
// 1. a future date that has most matches
// 2. a passed date that has most matches
// 3. any date found from document.
func (d *DocumentRule) extractDates(condition *models.RuleCondition, now time.Time, logger *logrus.Logger) (bool, error) {
	re, err := regexp.Compile(condition.Value)
	if err != nil {
		return false, fmt.Errorf("regex: %v", err)
	}
	nameMatches := re.FindAllString(d.Document.Name, -1)
	matches := re.FindAllString(d.Document.Content, -1)
	matches = append(nameMatches, matches...)

	if logger != nil {
		logger.Infof("Regex resulted in total of %d matches", len(matches))

		if len(matches) > 10 {
			logger.Infof("regex matches (first 10): %v", matches[0:9])
		} else {
			logger.Infof("regex matches: %v", matches)
		}

	}

	dates := make(map[time.Time]int)
	datesPassed := make(map[time.Time]int)
	datesUpcoming := make(map[time.Time]int)

	putDateToMap := func(date time.Time, m *map[time.Time]int) {
		if (*m)[date] == 0 {
			(*m)[date] = 1
		} else {
			(*m)[date] += 1
		}
	}

	for _, v := range matches {
		date, err := time.Parse(condition.DateFmt, v)
		if err != nil {
			logrus.Debugf("text %s does not match date fmt %s", v, condition.DateFmt)
			if logger != nil {
				logger.Warnf("matched text '%s' does not match date format '%s', skipping", v, condition.DateFmt)
			}
		} else {
			putDateToMap(date, &dates)
			if date.After(now) {
				putDateToMap(date, &datesUpcoming)
			} else {
				putDateToMap(date, &datesPassed)
			}
		}
	}

	if len(dates) == 0 {
		return false, nil
	}

	if logger != nil {
		logger.Infof("Found total of %d valid dates", len(dates))
	}

	pickDate := time.Time{}
	pickFrequency := 0
	for date, freq := range datesUpcoming {
		if freq > pickFrequency {
			pickDate = date
			pickFrequency = freq
		}
	}

	if len(datesUpcoming) == 0 {
		for date, freq := range datesPassed {
			if freq > pickFrequency {
				pickDate = date
				pickFrequency = freq
			}
		}
	}

	if logger != nil {
		logger.Infof("selected date %s", pickDate.String())
	}
	d.date = pickDate
	return true, nil
}

func (d *DocumentRule) RunActions() error {
	logrus.Debugf("execute rule %d actions for document: %s", d.Rule.Id, d.Document.Id)

	var err error
	var actionError error

	for i, action := range d.Rule.Actions {
		if !action.Enabled {
			logrus.Infof("rule %d action: %d (id:%d), type: %s disabled", d.Rule.Id, i, action.Id, action.Action)
			continue
		}
		logrus.Infof("run rule %d action: %d (id:%d), type: %s", d.Rule.Id, i, action.Id, action.Action)
		switch action.Action {
		case models.RuleActionSetName:
			actionError = d.setName(action)
		case models.RuleActionAppendName:
			actionError = d.appendName(action)
		case models.RuleActionSetDescription:
			actionError = d.setDescription(action)
		case models.RuleActionAppendDescription:
			actionError = d.appendDescription(action)
		case models.RuleActionAddMetadata:
			actionError = addMetadata(d.Document, int(action.MetadataKey), int(action.MetadataValue))
		case models.RuleActionRemoveMetadata:
			removeMetadata(d.Document, int(action.MetadataKey), int(action.MetadataValue))
		case models.RuleActionSetDate:
			actionError = d.setDate(action)
		default:
			e := errors.ErrInternalError
			e.ErrMsg = fmt.Sprintf("unknown action type: %v", action.Action)
			actionError = e
		}

		if actionError != nil {
			err = fmt.Errorf("action (%d): %v", action.Id, actionError)
			actionError = nil
		}
	}
	return err
}

func (d *DocumentRule) setName(action *models.RuleAction) error {
	d.Document.Name = action.Value
	return nil
}

func (d *DocumentRule) appendName(action *models.RuleAction) error {
	if !strings.HasSuffix(d.Document.Name, action.Value) {
		d.Document.Name += action.Value
	}
	return nil
}

func (d *DocumentRule) setDescription(action *models.RuleAction) error {
	d.Document.Description = action.Value
	return nil
}

func (d *DocumentRule) appendDescription(action *models.RuleAction) error {
	if !strings.HasSuffix(d.Document.Description, action.Value) {
		d.Document.Description += action.Value
	}
	return nil
}

func addMetadata(doc *models.Document, key, value int) error {
	if len(doc.Metadata) == 0 {
		doc.Metadata = []models.Metadata{{
			KeyId:   key,
			ValueId: value,
		}}
		return nil
	}

	// check if key-value already exists
	for _, v := range doc.Metadata {
		if v.KeyId == key && v.ValueId == value {
			return nil
		}
	}

	doc.Metadata = append(doc.Metadata, models.Metadata{
		KeyId:   key,
		ValueId: value,
	})
	return nil
}

// remove metadata. If valueId == 0, delete all metadata that matches the key.
func removeMetadata(doc *models.Document, keyId, valueId int) {
	i := 0
	for {
		if i > len(doc.Metadata)-1 {
			// all metadata was deleted
			break
		}
		if doc.Metadata[i].KeyId == keyId && (valueId == 0 || doc.Metadata[i].ValueId == valueId) {
			if i == len(doc.Metadata)-1 {
				// last item
				doc.Metadata = doc.Metadata[:i]
				break
			} else if i == 0 {
				// first item
				doc.Metadata = doc.Metadata[i+1:]
			} else {
				doc.Metadata = append(doc.Metadata[:i], doc.Metadata[i+1:]...)
			}
		} else {
			i += 1
		}
	}
}

func (d *DocumentRule) setDate(action *models.RuleAction) error {
	if !d.date.IsZero() {
		d.Document.Date = d.date
	}
	return nil
}

func matchTextAllowTypo(match, text string, matchPrefix, matchIs bool) (bool, error) {
	// max typos affect greatly the number of false positives, so try to be conservative with them..
	maxTypos := 0
	if len(match) > 30 {
		maxTypos = 3
	}
	if len(match) > 20 {
		maxTypos = 2
	} else if len(match) > 10 {
		maxTypos = 1
	}

	if maxTypos == 0 {
		if matchPrefix {
			return strings.HasPrefix(text, match), nil
		}
		if matchIs {
			return text == match, nil
		}
		return strings.Contains(text, match), nil
	}

	return matchTextByDistance(match, text, maxTypos, matchPrefix, matchIs)
}

func matchTextByDistance(match, text string, maxTypos int, matchPrefix, matchIs bool) (bool, error) {
	if len(match) < 2 || len(text) < 2 {
		return false, nil
	}
	if len(match) > len(text) {
		return false, nil
	}

	// compare match and text, allowing maxTypos of difference between texts.
	matchRunes := []rune(match)
	textRunes := []rune(text)
	matchIndex := 0
	typos := 0

	for i, r := range textRunes {
		if matchIs && matchIndex == len(matchRunes)-1 && matchIndex < len(matchRunes)-1 {
			// text continues after match
			return false, nil
		}

		if matchIs && matchIndex == len(matchRunes)-1 && i < len(textRunes)-1 {
			// match sequence completed, but there's still text left, no match
			return false, nil
		}

		if matchIndex >= len(matchRunes)-1 {
			// found match
			return true, nil
		}
		if matchIndex > 0 {
			// inside match sequence
			if matchRunes[matchIndex] == r {
				// next character
				matchIndex += 1
			} else {
				// no match
				typos += 1

				if matchRunes[matchIndex+1] == r {
					// if text is missing one character, skip match character as well and
					matchIndex += 1
					typos -= 1
				} else if i < len(textRunes)-1 {
					// if text has one character too much, skip the character
					if matchRunes[matchIndex] == textRunes[i+1] {
						typos -= 1
					}
				}
				matchIndex += 1
				if typos > maxTypos {
					// match failed, reset
					typos = 0
					matchIndex = 0
				}
			}
		} else if matchRunes[0] == r {
			// start match
			matchIndex += 1
		}

		if matchPrefix && matchIndex == 0 && i > 0 {
			// match didn't start from beginning
			return false, nil
		}
	}
	return false, nil
}

func matchMetadata(document *models.Document, values *[]models.MetadataValue) error {
	logrus.Debugf("match metadata keys for doc: %s, %d rules", document.Id, len(*values))
	for _, v := range *values {
		match, err := documentMatchesFilter(document, v.MatchType, v.MatchFilter)
		if err != nil {
			logrus.Debugf("automatic metadata rule, filter error: %v", err)
			continue
		}
		if match != "" {
			addMetadata(document, v.KeyId, v.Id)
		}
	}
	return nil
}

var reRegexHasSubMatch = regexp.MustCompile("\\(.+\\)")

func documentMatchesFilter(document *models.Document, ruleType models.MetadataRuleType, filter string) (string, error) {
	if ruleType == models.MetadataMatchExact {
		lowerContent := strings.ToLower(document.Content)
		lowerRule := strings.ToLower(filter)
		contains, err := matchTextAllowTypo(lowerRule, lowerContent, false, false)
		if contains {
			return lowerRule, err
		} else {
			return "", err
		}
	} else if ruleType == models.MetadataMatchRegex {
		// if regex captures submatch, return first submatch (not the match itself),
		// else return regex match
		re, err := regexp.Compile(filter)
		if err != nil {
			return "", fmt.Errorf("invalid regex: %v", err)
		}

		if reRegexHasSubMatch.MatchString(filter) {
			matches := re.FindStringSubmatch(document.Content)
			if len(matches) == 0 {
				return "", nil
			}
			if len(matches) == 1 {
				return "", nil
			}

			if len(matches) == 2 {
				return matches[1], nil
			} else {
				logrus.Debugf("more than 1 regex matches, pick first. regex: %s doc. %s, matches: %v",
					filter, document.Id, matches)
				return matches[1], nil
			}
		} else {
			match := re.FindString(filter)
			return match, nil
		}
	} else {
		return "", fmt.Errorf("unknown rule type: %s", ruleType)
	}
}
