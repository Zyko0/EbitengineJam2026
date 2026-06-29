//go:build !dev

package assets

import (
	_ "embed"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
)

var (
	//go:embed shaders/scene.kage
	shaderSceneSrc []byte
	shaderScene    *ebiten.Shader

	//go:embed shaders/post_process.kage
	shaderPostProcessSrc []byte
	shaderPostProcess    *ebiten.Shader

	//go:embed shaders/entity.kage
	shaderEntitySrc []byte
	shaderEntity    *ebiten.Shader
)

func init() {
	var err error

	shaderScene, err = ebiten.NewShader(shaderSceneSrc)
	if err != nil {
		log.Fatal("shader scene: ", err)
	}
	shaderPostProcess, err = ebiten.NewShader(shaderPostProcessSrc)
	if err != nil {
		log.Fatal("shader post_process: ", err)
	}
	shaderEntity, err = ebiten.NewShader(shaderEntitySrc)
	if err != nil {
		log.Fatal("shader entity: ", err)
	}
}

func ShaderScene() *ebiten.Shader {
	return shaderScene
}

func ShaderPostProcess() *ebiten.Shader {
	return shaderPostProcess
}

func ShaderEntity() *ebiten.Shader {
	return shaderEntity
}
