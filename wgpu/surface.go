//go:build !js

package wgpu

/*
#include <stdlib.h>
#include "gen_wgpu_wrappers.h"
*/
import "C"
import (
	"errors"
	"runtime"
	"unsafe"
)

func (g *Surface) GetCapabilities(adapter *Adapter) (ret SurfaceCapabilities) {
	var caps C.WGPUSurfaceCapabilities
	C.wgpuSurfaceGetCapabilities(g.ref, adapter.ref, &caps)

	if caps.alphaModeCount == 0 && caps.formatCount == 0 && caps.presentModeCount == 0 {
		return
	}
	if caps.formatCount > 0 {
		caps.formats = (*C.WGPUTextureFormat)(C.calloc(C.size_t(unsafe.Sizeof(C.WGPUTextureFormat(0))), caps.formatCount))
		defer C.free(unsafe.Pointer(caps.formats))
	}
	if caps.presentModeCount > 0 {
		caps.presentModes = (*C.WGPUPresentMode)(C.calloc(C.size_t(unsafe.Sizeof(C.WGPUPresentMode(0))), caps.presentModeCount))
		defer C.free(unsafe.Pointer(caps.presentModes))
	}
	if caps.alphaModeCount > 0 {
		caps.alphaModes = (*C.WGPUCompositeAlphaMode)(C.calloc(C.size_t(unsafe.Sizeof(C.WGPUCompositeAlphaMode(0))), caps.alphaModeCount))
		defer C.free(unsafe.Pointer(caps.alphaModes))
	}

	C.wgpuSurfaceGetCapabilities(g.ref, adapter.ref, &caps)

	if caps.formatCount > 0 {
		formatsTmp := unsafe.Slice((*TextureFormat)(caps.formats), caps.formatCount)
		ret.Formats = make([]TextureFormat, caps.formatCount)
		copy(ret.Formats, formatsTmp)
	}
	if caps.presentModeCount > 0 {
		presentModesTmp := unsafe.Slice((*PresentMode)(caps.presentModes), caps.presentModeCount)
		ret.PresentModes = make([]PresentMode, caps.presentModeCount)
		copy(ret.PresentModes, presentModesTmp)
	}
	if caps.alphaModeCount > 0 {
		alphaModesTmp := unsafe.Slice((*CompositeAlphaMode)(caps.alphaModes), caps.alphaModeCount)
		ret.AlphaModes = make([]CompositeAlphaMode, caps.alphaModeCount)
		copy(ret.AlphaModes, alphaModesTmp)
	}

	return
}

func (g *Surface) Configure(device *Device, config *SurfaceConfiguration) {
	if g.device != nil {
		// release previously referenced device
		C.wgpuDeviceRelease(g.device)
		g.device = nil
	}

	g.device = device.addRef()

	var pinner runtime.Pinner
	defer pinner.Unpin()

	var cfg *C.WGPUSurfaceConfiguration
	if config != nil {
		var nextInChain *C.WGPUSurfaceConfigurationExtras

		if config.DesiredMaximumFrameLatency > 0 {
			nextInChain = &C.WGPUSurfaceConfigurationExtras{
				chain: C.WGPUChainedStruct{
					sType: C.WGPUSType_SurfaceConfigurationExtras,
				},
				desiredMaximumFrameLatency: C.uint32_t(config.DesiredMaximumFrameLatency),
			}

			pinner.Pin(nextInChain)
		}

		cfg = &C.WGPUSurfaceConfiguration{
			device:      g.device,
			format:      C.WGPUTextureFormat(config.Format),
			usage:       C.WGPUTextureUsage(config.Usage),
			alphaMode:   C.WGPUCompositeAlphaMode(config.AlphaMode),
			width:       C.uint32_t(config.Width),
			height:      C.uint32_t(config.Height),
			presentMode: C.WGPUPresentMode(config.PresentMode),
			nextInChain: (*C.WGPUChainedStruct)(unsafe.Pointer(nextInChain)),
		}

		if len(config.ViewFormats) > 0 {
			pinner.Pin(&config.ViewFormats[0])

			cfg.viewFormatCount = C.size_t(len(config.ViewFormats))
			cfg.viewFormats = (*C.WGPUTextureFormat)(&config.ViewFormats[0])
		}
	}

	C.wgpuSurfaceConfigure(g.ref, cfg)
}

// NOTE: you should typically not call [Texture.Release] on the returned texture.
// Instead, you should call [TextureView.Release] on any [TextureView] you create from it.
// You need to check the status of the returned texture, or use SurfaceTexture.Get.
func (g *Surface) TryGetCurrentTexture() (SurfaceTexture, error) {
	if g.device == nil {
		return SurfaceTexture{}, errors.New("surface not configured")
	}

	errh := acquireErrorCallback()
	defer errh.Done()

	var surfaceTexture C.WGPUSurfaceTexture
	C.go_wgpuSurfaceGetCurrentTexture(
		g.device,
		errh.ToPointer(),
		g.ref,
		&surfaceTexture,
	)
	if err := errh.ToError(); err != nil {
		if surfaceTexture.texture != nil {
			C.wgpuTextureRelease(surfaceTexture.texture)
		}

		return SurfaceTexture{}, err
	}

	status := SurfaceGetCurrentTextureStatus(surfaceTexture.status)

	var texture *Texture
	if surfaceTexture.texture != nil {
		C.wgpuDeviceAddRef(g.device)
		texture = &Texture{device: g.device, ref: surfaceTexture.texture}
	}

	return SurfaceTexture{Texture: texture, Status: status}, nil
}

func (g *Surface) Present() {
	C.wgpuSurfacePresent(g.ref)
}
