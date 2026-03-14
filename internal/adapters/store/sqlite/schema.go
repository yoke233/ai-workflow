package sqlite

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

func autoMigrate(ctx context.Context, orm *gorm.DB) error {
	if orm == nil {
		return fmt.Errorf("nil orm")
	}
	return orm.WithContext(ctx).AutoMigrate(
		&ProjectModel{},
		&ResourceBindingModel{},
		&WorkItemModel{},
		&ActionModel{},
		&RunModel{},
		&AgentContextModel{},
		&EventModel{},
		&AgentProfileModel{},
		&DAGTemplateModel{},
		&UsageRecordModel{},
		&ThreadModel{},
		&ThreadMessageModel{},
		&ThreadMemberModel{},
		&ThreadWorkItemLinkModel{},
		&FeatureEntryModel{},
		&ActionSignalModel{},
		&ActionResourceModel{},
		&InspectionReportModel{},
		&InspectionFindingModel{},
		&InspectionInsightModel{},
		&NotificationModel{},
	)
}
