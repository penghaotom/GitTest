package main

import (
	"encoding/json"
	"fmt"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"

)

const car = "car~type"
const temp = "TempVehicle"
const formal = "FormalVehicle"
const voteThreshold = 2

//Smartcontract provides the function for managing
type SmartContract struct {
	contractapi.Contract
}
//发起人身份信息
type TempVehicle struct{
	Id 					string 		`json:"id"`						//临时车辆名称
	Threshold			int		`json:"threshold"`					//临时车辆成功注册所需要的背书者的数量
	EndorserList		[]string	`json:"endorser_list"`			//为白板车申请身份背书的车辆Id列表
}
type FormalVehicle struct {
	Id 					string		`json:"id"`						//正式车辆名称
	Uid					string		`json:"uid"`					//车辆名称所对应的SDK的wallet中的ID
	VoteWeight			int		`json:"vote_weight"`				//车辆投票权重
	EndorserList		[]string	`json:"endorser_list"`			//加入网络时的背书车辆的列表
}

func createCarCompositeKey(ctx contractapi.TransactionContextInterface,attribute ,id string) (string, error){
	attr := []string{attribute,id}
	compositeKey, err := ctx.GetStub().CreateCompositeKey(car, attr)
	if err != nil{
		return "",fmt.Errorf("the composite key failed to be created")
	}
	return compositeKey, nil
}


//获得SDK的user身份ID，每个user的msp有一个唯一身份ID
func (s *SmartContract) GetSDKuserId(ctx contractapi.TransactionContextInterface) string {
	uid, err := ctx.GetClientIdentity().GetID()
	if err != nil {
		return ""
	}
	return uid
}

//向账本中添加临时车辆表项
func (s *SmartContract) CreateTempVehicle(ctx contractapi.TransactionContextInterface, id , userId string ) (bool,error){
	//创建复合键名
	compositeKey, err := createCarCompositeKey(ctx,temp,id)
	if err != nil {
		return false, err
	}
	//创建临时车辆表项
	newTempCar := &TempVehicle{
		Id:           compositeKey,
		Threshold:    1,
		EndorserList: []string{userId},
	}
	newTempCarJson,err := json.Marshal(newTempCar)
	if err != nil{
		return false, fmt.Errorf("JSON format conversion failed")
	}
	err = ctx.GetStub().PutState(compositeKey, newTempCarJson)
	if err != nil{
		return false, err
	}
	return true, nil
}
//创建正式车辆表项
func (s *SmartContract) TransToFormalVehicle(ctx contractapi.TransactionContextInterface, id string) error{

	//检测该车辆是否已经存在于正式车辆表项中
	if ok, _ := s.IsCompositeExisted(ctx, car,formal, id); ok == true {
		return fmt.Errorf("The car is not a legal formal vehicle")
	}
	//创建复合键名,搜索临时表项
	tempCompositeKey, err := createCarCompositeKey(ctx, temp ,id)
	if err != nil {
		return err
	}
	tempCar, _ := s.GetTempCar(ctx, tempCompositeKey)
	//创建正式车辆表项
	newFormalCar := &FormalVehicle{
		Id:           tempCar.Id,
		Uid:          "",
		VoteWeight:   2,
		EndorserList: tempCar.EndorserList,
	}
	//转化为Json格式添加账本
	newFormalCarJson,err := json.Marshal(newFormalCar)
	if err != nil{
		return fmt.Errorf("JSON format conversion failed")
	}
	formalCompositeKey, err := createCarCompositeKey(ctx, formal ,id)
	err = ctx.GetStub().PutState(formalCompositeKey, newFormalCarJson)
	if err != nil{
		return err
	}
	return  nil
}
//检查账本中是否存在某一复合键
func (s *SmartContract) IsCompositeExisted(ctx contractapi.TransactionContextInterface, objectType string, attributes string, id string) (bool, error) {
	attr := []string{attributes,id}
	compositeKey, err := ctx.GetStub().CreateCompositeKey(objectType, attr)
	if err != nil{
		return false,fmt.Errorf("the composite key failed to be created")
	}
	tempJson, err := ctx.GetStub().GetState(compositeKey)
	return tempJson != nil, err
}

func (s *SmartContract) GetSDKcreatorId(ctx contractapi.TransactionContextInterface) string{
	uid, err := ctx.GetStub().GetCreator()
	if err != nil {
		return ""
	}
	return string(uid)
}

//为临时车辆投票
func (s *SmartContract) Vote(ctx contractapi.TransactionContextInterface, id string) (bool, error) {
	userId := s.GetSDKuserId(ctx)
	//临时表项存在
	if tempCar, _ := s.GetTempCar(ctx, id); tempCar != nil{
		for _, endorser := range tempCar.EndorserList{
			if endorser == userId {
				return false, fmt.Errorf("该正式车已投票")
			}
		}
		//投票阈值大于0
		if tempCar.Threshold > 0 {
			tempCar.EndorserList = append(tempCar.EndorserList, userId)
			tempCar.Threshold = tempCar.Threshold - 1
			carJson, _ := json.Marshal(tempCar)
			compositeKey, _ := createCarCompositeKey(ctx, temp , id)
			_ = ctx.GetStub().PutState(compositeKey, carJson)
			if tempCar.Threshold == 0 {
				return true , nil
			}
			return false, nil
		}
		return false, nil
	}
	//临时表项不存在
	_, err := s.CreateTempVehicle(ctx, id, userId)
	if err != nil{
		return false,fmt.Errorf("创建临时表项失败")
	}
	return false , nil
}
func (s *SmartContract) InitFormalVehicle(ctx contractapi.TransactionContextInterface, id string) (*FormalVehicle, error){
	userId := s.GetSDKuserId(ctx)
	if ok, _ := s.IsCompositeExisted(ctx, car,formal, id); ok != false {
		return &FormalVehicle{},fmt.Errorf("The car has been created")
	}
	//创建复合键名,搜索临时表项
	tempCompositeKey, err := createCarCompositeKey(ctx, formal ,id)
	if err != nil {
		return &FormalVehicle{}, err
	}
	newFormalVehicle := &FormalVehicle{
		Id:           tempCompositeKey,
		Uid:          userId,
		VoteWeight:   1,
		EndorserList: []string{},
	}
	formalVehicleJson,err  := json.Marshal(newFormalVehicle)
	if err != nil{
		return &FormalVehicle{},nil
	}
	_ = ctx.GetStub().PutState(tempCompositeKey,formalVehicleJson)
	return newFormalVehicle,nil
}

// GetAsset returns the basic asset with id given from the world state
func (s *SmartContract) GetTempCar(ctx contractapi.TransactionContextInterface, key string) (*TempVehicle, error) {
	compositeKey, err := createCarCompositeKey(ctx, temp , key )
	if err != nil {
		return  nil, err
	}
	existing, err:= ctx.GetStub().GetState(compositeKey)
	if existing == nil {
		return nil, fmt.Errorf("Cannot read world state pair with key %s. Does not exist", key)
	}
	ba := new(TempVehicle)

	err = json.Unmarshal(existing, ba)

	if err != nil {
		return nil, fmt.Errorf("Data retrieved from world state for key %s was not of type Voter", key)
	}
	return ba, nil
}

func (s *SmartContract) GetFormalCar(ctx contractapi.TransactionContextInterface, key string) (*FormalVehicle, error) {
	compositeKey, err := createCarCompositeKey(ctx, formal , key)
	if err != nil {
		return nil,err
	}

	existing, err:= ctx.GetStub().GetState(compositeKey)

	if existing == nil {
		return nil, fmt.Errorf("Cannot read world state pair with key %s. Does not exist", key)
	}

	ba := new(FormalVehicle)

	err = json.Unmarshal(existing, ba)

	if err != nil {
		return nil, fmt.Errorf("Data retrieved from world state for key %s was not of type Voter", key)
	}
	return ba, nil
}

func main() {

	chaincode, err := contractapi.NewChaincode(new(SmartContract))

	if err != nil {
		fmt.Printf("Error create fabcar chaincode: %s", err.Error())
		return
	}

	if err := chaincode.Start(); err != nil {
		fmt.Printf("Error starting fabcar chaincode: %s", err.Error())
	}
}

