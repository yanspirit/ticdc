// Copyright 2021 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package owner

import (
	"github.com/pingcap/check"
	"github.com/pingcap/ticdc/cdc/model"
	"github.com/pingcap/ticdc/pkg/config"
	cdcContext "github.com/pingcap/ticdc/pkg/context"
	"github.com/pingcap/ticdc/pkg/orchestrator"
	"github.com/pingcap/ticdc/pkg/util/testleak"
)

var _ = check.Suite(&feedStateManagerSuite{})

type feedStateManagerSuite struct {
}

func (s *feedStateManagerSuite) TestHandleJob(c *check.C) {
	defer testleak.AfterTest(c)()
	ctx := cdcContext.NewBackendContext4Test(true)
	manager := new(feedStateManager)
	state := model.NewChangefeedReactorState(ctx.ChangefeedVars().ID)
	tester := orchestrator.NewReactorStateTester(c, state, nil)
	state.PatchInfo(func(info *model.ChangeFeedInfo) (*model.ChangeFeedInfo, bool, error) {
		c.Assert(info, check.IsNil)
		return &model.ChangeFeedInfo{SinkURI: "123", Config: &config.ReplicaConfig{}}, true, nil
	})
	state.PatchStatus(func(status *model.ChangeFeedStatus) (*model.ChangeFeedStatus, bool, error) {
		c.Assert(status, check.IsNil)
		return &model.ChangeFeedStatus{}, true, nil
	})
	manager.Tick(state)
	tester.MustApplyPatches()
	c.Assert(manager.ShouldRunning(), check.IsTrue)

	// an admin job which of changefeed is not match
	manager.PushAdminJob(&model.AdminJob{
		CfID: "fake-changefeed-id",
		Type: model.AdminStop,
	})
	manager.Tick(state)
	tester.MustApplyPatches()
	c.Assert(manager.ShouldRunning(), check.IsTrue)

	// a running can not be resume
	manager.PushAdminJob(&model.AdminJob{
		CfID: ctx.ChangefeedVars().ID,
		Type: model.AdminResume,
	})
	manager.Tick(state)
	tester.MustApplyPatches()
	c.Assert(manager.ShouldRunning(), check.IsTrue)

	// stop a changefeed
	manager.PushAdminJob(&model.AdminJob{
		CfID: ctx.ChangefeedVars().ID,
		Type: model.AdminStop,
	})
	manager.Tick(state)
	tester.MustApplyPatches()
	c.Assert(manager.ShouldRunning(), check.IsFalse)
	c.Assert(state.Info.State, check.Equals, model.StateStopped)
	c.Assert(state.Info.AdminJobType, check.Equals, model.AdminStop)
	c.Assert(state.Status.AdminJobType, check.Equals, model.AdminStop)

	// resume a changefeed
	manager.PushAdminJob(&model.AdminJob{
		CfID: ctx.ChangefeedVars().ID,
		Type: model.AdminResume,
	})
	manager.Tick(state)
	tester.MustApplyPatches()
	c.Assert(manager.ShouldRunning(), check.IsTrue)
	c.Assert(state.Info.State, check.Equals, model.StateNormal)
	c.Assert(state.Info.AdminJobType, check.Equals, model.AdminNone)
	c.Assert(state.Status.AdminJobType, check.Equals, model.AdminNone)

	// remove a changefeed
	manager.PushAdminJob(&model.AdminJob{
		CfID: ctx.ChangefeedVars().ID,
		Type: model.AdminRemove,
	})
	manager.Tick(state)
	tester.MustApplyPatches()
	c.Assert(manager.ShouldRunning(), check.IsFalse)
	c.Assert(state.Info.State, check.Equals, model.StateRemoved)
	c.Assert(state.Info.AdminJobType, check.Equals, model.AdminRemove)
	c.Assert(state.Status.AdminJobType, check.Equals, model.AdminRemove)

	// a removed changefeed can not be stop
	manager.PushAdminJob(&model.AdminJob{
		CfID: ctx.ChangefeedVars().ID,
		Type: model.AdminStop,
	})
	manager.Tick(state)
	tester.MustApplyPatches()
	c.Assert(manager.ShouldRunning(), check.IsFalse)
	c.Assert(state.Info.State, check.Equals, model.StateRemoved)
	c.Assert(state.Info.AdminJobType, check.Equals, model.AdminRemove)
	c.Assert(state.Status.AdminJobType, check.Equals, model.AdminRemove)

	// force remove a changefeed
	manager.PushAdminJob(&model.AdminJob{
		CfID: ctx.ChangefeedVars().ID,
		Type: model.AdminRemove,
		Opts: &model.AdminJobOption{ForceRemove: true},
	})
	manager.Tick(state)
	tester.MustApplyPatches()
	c.Assert(manager.ShouldRunning(), check.IsFalse)
	c.Assert(state.Info, check.IsNil)
	c.Assert(state.Status, check.IsNil)
}

func (s *feedStateManagerSuite) TestMarkFinished(c *check.C) {
	defer testleak.AfterTest(c)()
	ctx := cdcContext.NewBackendContext4Test(true)
	manager := new(feedStateManager)
	state := model.NewChangefeedReactorState(ctx.ChangefeedVars().ID)
	tester := orchestrator.NewReactorStateTester(c, state, nil)
	state.PatchInfo(func(info *model.ChangeFeedInfo) (*model.ChangeFeedInfo, bool, error) {
		c.Assert(info, check.IsNil)
		return &model.ChangeFeedInfo{SinkURI: "123", Config: &config.ReplicaConfig{}}, true, nil
	})
	state.PatchStatus(func(status *model.ChangeFeedStatus) (*model.ChangeFeedStatus, bool, error) {
		c.Assert(status, check.IsNil)
		return &model.ChangeFeedStatus{}, true, nil
	})
	manager.Tick(state)
	tester.MustApplyPatches()
	c.Assert(manager.ShouldRunning(), check.IsTrue)

	manager.MarkFinished()
	manager.Tick(state)
	tester.MustApplyPatches()
	c.Assert(manager.ShouldRunning(), check.IsFalse)
	c.Assert(state.Info.State, check.Equals, model.StateFinished)
	c.Assert(state.Info.AdminJobType, check.Equals, model.AdminFinish)
	c.Assert(state.Status.AdminJobType, check.Equals, model.AdminFinish)
}

func (s *feedStateManagerSuite) TestCleanUpInfos(c *check.C) {
	defer testleak.AfterTest(c)()
	ctx := cdcContext.NewBackendContext4Test(true)
	manager := new(feedStateManager)
	state := model.NewChangefeedReactorState(ctx.ChangefeedVars().ID)
	tester := orchestrator.NewReactorStateTester(c, state, nil)
	state.PatchInfo(func(info *model.ChangeFeedInfo) (*model.ChangeFeedInfo, bool, error) {
		c.Assert(info, check.IsNil)
		return &model.ChangeFeedInfo{SinkURI: "123", Config: &config.ReplicaConfig{}}, true, nil
	})
	state.PatchStatus(func(status *model.ChangeFeedStatus) (*model.ChangeFeedStatus, bool, error) {
		c.Assert(status, check.IsNil)
		return &model.ChangeFeedStatus{}, true, nil
	})
	state.PatchTaskStatus(ctx.GlobalVars().CaptureInfo.ID, func(status *model.TaskStatus) (*model.TaskStatus, bool, error) {
		return &model.TaskStatus{}, true, nil
	})
	state.PatchTaskPosition(ctx.GlobalVars().CaptureInfo.ID, func(position *model.TaskPosition) (*model.TaskPosition, bool, error) {
		return &model.TaskPosition{}, true, nil
	})
	state.PatchTaskWorkload(ctx.GlobalVars().CaptureInfo.ID, func(workload model.TaskWorkload) (model.TaskWorkload, bool, error) {
		return model.TaskWorkload{}, true, nil
	})
	tester.MustApplyPatches()
	c.Assert(state.TaskStatuses, check.HasKey, ctx.GlobalVars().CaptureInfo.ID)
	c.Assert(state.TaskPositions, check.HasKey, ctx.GlobalVars().CaptureInfo.ID)
	c.Assert(state.Workloads, check.HasKey, ctx.GlobalVars().CaptureInfo.ID)
	manager.Tick(state)
	tester.MustApplyPatches()
	c.Assert(manager.ShouldRunning(), check.IsTrue)

	manager.MarkFinished()
	manager.Tick(state)
	tester.MustApplyPatches()
	c.Assert(manager.ShouldRunning(), check.IsFalse)
	c.Assert(state.Info.State, check.Equals, model.StateFinished)
	c.Assert(state.Info.AdminJobType, check.Equals, model.AdminFinish)
	c.Assert(state.Status.AdminJobType, check.Equals, model.AdminFinish)
	c.Assert(state.TaskStatuses, check.Not(check.HasKey), ctx.GlobalVars().CaptureInfo.ID)
	c.Assert(state.TaskPositions, check.Not(check.HasKey), ctx.GlobalVars().CaptureInfo.ID)
	c.Assert(state.Workloads, check.Not(check.HasKey), ctx.GlobalVars().CaptureInfo.ID)
}

func (s *feedStateManagerSuite) TestHandleError(c *check.C) {
	defer testleak.AfterTest(c)()
	ctx := cdcContext.NewBackendContext4Test(true)
	manager := new(feedStateManager)
	state := model.NewChangefeedReactorState(ctx.ChangefeedVars().ID)
	tester := orchestrator.NewReactorStateTester(c, state, nil)
	state.PatchInfo(func(info *model.ChangeFeedInfo) (*model.ChangeFeedInfo, bool, error) {
		c.Assert(info, check.IsNil)
		return &model.ChangeFeedInfo{SinkURI: "123", Config: &config.ReplicaConfig{}}, true, nil
	})
	state.PatchStatus(func(status *model.ChangeFeedStatus) (*model.ChangeFeedStatus, bool, error) {
		c.Assert(status, check.IsNil)
		return &model.ChangeFeedStatus{}, true, nil
	})
	state.PatchTaskStatus(ctx.GlobalVars().CaptureInfo.ID, func(status *model.TaskStatus) (*model.TaskStatus, bool, error) {
		return &model.TaskStatus{}, true, nil
	})
	state.PatchTaskPosition(ctx.GlobalVars().CaptureInfo.ID, func(position *model.TaskPosition) (*model.TaskPosition, bool, error) {
		return &model.TaskPosition{Error: &model.RunningError{
			Addr:    ctx.GlobalVars().CaptureInfo.AdvertiseAddr,
			Code:    "[CDC:ErrEtcdSessionDone]",
			Message: "fake error for test",
		}}, true, nil
	})
	state.PatchTaskWorkload(ctx.GlobalVars().CaptureInfo.ID, func(workload model.TaskWorkload) (model.TaskWorkload, bool, error) {
		return model.TaskWorkload{}, true, nil
	})
	tester.MustApplyPatches()
	manager.Tick(state)
	tester.MustApplyPatches()
	c.Assert(manager.ShouldRunning(), check.IsTrue)
	// error reported by processor in task position should be cleaned
	c.Assert(state.TaskPositions[ctx.GlobalVars().CaptureInfo.ID].Error, check.IsNil)

	// throw error more than history threshold to turn feed state into error
	for i := 0; i < model.ErrorHistoryThreshold; i++ {
		state.PatchTaskPosition(ctx.GlobalVars().CaptureInfo.ID, func(position *model.TaskPosition) (*model.TaskPosition, bool, error) {
			return &model.TaskPosition{Error: &model.RunningError{
				Addr:    ctx.GlobalVars().CaptureInfo.AdvertiseAddr,
				Code:    "[CDC:ErrEtcdSessionDone]",
				Message: "fake error for test",
			}}, true, nil
		})
		tester.MustApplyPatches()
		manager.Tick(state)
		tester.MustApplyPatches()
	}
	c.Assert(manager.ShouldRunning(), check.IsFalse)
	c.Assert(state.Info.State, check.Equals, model.StateError)
	c.Assert(state.Info.AdminJobType, check.Equals, model.AdminStop)
	c.Assert(state.Status.AdminJobType, check.Equals, model.AdminStop)
}
