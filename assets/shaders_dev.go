//go:build dev

package assets

import (
	"fmt"
	"log"

	"github.com/Zyko0/Ebiary/asset"
	"github.com/hajimehoshi/ebiten/v2"
)

var (
	shaderScene       *asset.LiveAsset[*ebiten.Shader]
	shaderPostProcess *asset.LiveAsset[*ebiten.Shader]
	shaderEntity      *asset.LiveAsset[*ebiten.Shader]
)

func init() {
	var err error

	shaderScene, err = asset.NewLiveAsset[*ebiten.Shader]("assets/shaders/scene.kage")
	if err != nil {
		log.Fatal("shader scene: ", err)
	}
	shaderPostProcess, err = asset.NewLiveAsset[*ebiten.Shader]("assets/shaders/post_process.kage")
	if err != nil {
		log.Fatal("shader post_process: ", err)
	}
	shaderEntity, err = asset.NewLiveAsset[*ebiten.Shader]("assets/shaders/entity.kage")
	if err != nil {
		log.Fatal("shader entity: ", err)
	}
}

func ShaderScene() *ebiten.Shader {
	if shaderScene.Error() != nil {
		fmt.Println("shader scene: ", shaderScene.Error())
	}
	return shaderScene.Value()
}

func ShaderPostProcess() *ebiten.Shader {
	if shaderPostProcess.Error() != nil {
		fmt.Println("shader post_process: ", shaderPostProcess.Error())
	}
	return shaderPostProcess.Value()
}

func ShaderEntity() *ebiten.Shader {
	if shaderEntity.Error() != nil {
		fmt.Println("shader entity: ", shaderEntity.Error())
	}
	return shaderEntity.Value()
}
