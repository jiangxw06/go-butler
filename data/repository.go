package data

const (
	//InsertModelOpType 新增操作
	InsertModelOpType ModelOpType = iota
	//UpdateModelOpType 更改操作
	UpdateModelOpType
	//DeleteModelOpType 删除操作
	DeleteModelOpType
)

type (
	//DaoMutation 是数据库DAO对象的改变
	DaoMutation struct {
		Model    interface{}
		OpType   ModelOpType
		OldModel interface{} // only for update op
	}

	//DaoEventPublisher 仅用于内部
	DaoEventPublisher interface {
		AddListener(listener DaoEventListener)
		Publish(event DaoMutation)
	}

	//DaoEventListener 仅用于内部
	DaoEventListener interface {
		Listen(event DaoMutation)
	}

	//ModelOpType 是数据库记录的变化类型
	ModelOpType int

	//RepoBase 是数据仓库的基础类型
	RepoBase struct {
		daoEventListeners []DaoEventListener
	}
)

var (
	_ DaoEventPublisher = new(RepoBase)
)

//AddListener 仅用于内部
func (r *RepoBase) AddListener(listener DaoEventListener) {
	r.daoEventListeners = append(r.daoEventListeners, listener)
}

//Notify 仅用于内部
func (r *RepoBase) Publish(mut DaoMutation) {
	for _, listener := range r.daoEventListeners {
		listener.Listen(mut)
	}
}

//Save 保存记录的更改
//func (r *RepoBase) Save(ctx context.Context, tx *gorm.DB, modelPtr interface{}) error {
//	res := tx.Save(modelPtr)
//	if res.Error != nil {
//		return res.Error
//	}
//	if res.RowsAffected != 1 {
//		//todo
//	}
//	r.Notify(ctx, []DaoMutation{
//		{
//			OpType:   UpdateModelOpType,
//			Model:    modelPtr,
//			OldModel: nil, //todo 根据需要查出更新前的旧对象
//		},
//	})
//	return nil
//}
