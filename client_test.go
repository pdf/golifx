package golifx_test

import (
	"errors"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/pdf/golifx"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	"github.com/pdf/golifx/common"
	"github.com/pdf/golifx/mock"
)

func init() {
	format.UseStringerRepresentation = false
}

var _ = Describe("Golifx", func() {
	var (
		mockCtrl     *gomock.Controller
		mockProtocol *mock.MockProtocol
		mockDevice   *mock.MockDevice
		mockLight    *mock.MockLight
		client       *Client
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockProtocol = mock.NewMockProtocol(mockCtrl)

		mockDevice = mock.NewMockDevice(mockCtrl)
		mockLight = mock.NewMockLight(mockCtrl)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	It("should send discovery to the protocol on NewClient", func() {
		gomock.InOrder(
			mockProtocol.EXPECT().SetClient(gomock.Any()),
			mockProtocol.EXPECT().Discover(),
		)
		client, err := NewClient(mockProtocol)
		Expect(client).To(BeAssignableToTypeOf(new(Client)))
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Client", func() {
		BeforeEach(func() {
			gomock.InOrder(
				mockProtocol.EXPECT().SetClient(gomock.Any()),
				mockProtocol.EXPECT().Discover(),
			)
			client, _ = NewClient(mockProtocol)
			client.SetTimeout(50 * time.Millisecond)
		})

		It("should update the timeout", func() {
			timeout := 5 * time.Second
			client.SetTimeout(timeout)
			Expect(client.GetTimeout()).To(Equal(&timeout))
		})

		It("should update the retry interval", func() {
			interval := 5 * time.Millisecond
			client.SetRetryInterval(interval)
			Expect(client.GetRetryInterval()).To(Equal(&interval))
		})

		It("should set the retry to half the timeout if it's >= the timeout", func() {
			timeout := 10 * time.Second
			halfTimeout := timeout / 2
			client.SetTimeout(timeout)
			interval := 10 * time.Second
			client.SetRetryInterval(interval)
			Expect(client.GetRetryInterval()).To(Equal(&halfTimeout))
		})

		It("should update the discovery interval", func() {
			interval := 5 * time.Second
			Expect(client.SetDiscoveryInterval(interval)).To(Succeed())
		})

		It("should update the discovery interval when it's non-zero", func() {
			interval := 5 * time.Second
			Expect(client.SetDiscoveryInterval(interval)).To(Succeed())
			interval = 10 * time.Second
			Expect(client.SetDiscoveryInterval(interval)).To(Succeed())
		})

		It("should send SetPower to the protocol", func() {
			mockProtocol.EXPECT().SetPower(true)
			client.SetPower(true)
		})

		It("should send SetPowerDuration to the protocol", func() {
			duration := 5 * time.Second
			mockProtocol.EXPECT().SetPowerDuration(true, duration)
			client.SetPowerDuration(true, duration)
		})

		It("should send SetColor to the protocol", func() {
			color := common.Color{}
			duration := 1 * time.Millisecond
			mockProtocol.EXPECT().SetColor(color, duration)
			client.SetColor(color, duration)
		})

		It("should return an error from GetDevices when it knows no devices", func() {
			devices, err := client.GetDevices()
			Expect(len(devices)).To(Equal(0))
			Expect(err).To(Equal(common.ErrNotFound))
		})

		It("should add a device", func() {
			mockDevice.EXPECT().ID().Return(uint64(1))
			Expect(client.AddDevice(mockDevice)).To(Succeed())
		})

		It("should close successfully", func() {
			mockProtocol.EXPECT().Close().Return(nil)
			Expect(client.Close()).To(Succeed())
		})

		It("should return an error on failed close", func() {
			mockProtocol.EXPECT().Close().Return(errors.New(`failed`))
			Expect(client.Close()).NotTo(Succeed())
		})

		Context("with devices", func() {
			var (
				deviceID          = uint64(1234)
				deviceUnkownID    = uint64(4321)
				deviceLabel       = `mockDevice`
				deviceUnkownLabel = `unknownDevice`
				lightID           = uint64(5678)
				lightLabel        = `mockLight`
			)

			BeforeEach(func() {
				mockDevice.EXPECT().ID().Return(deviceID)
				client.AddDevice(mockDevice)
			})

			Context("adding a device", func() {
				It("should return the device", func() {
					devices, err := client.GetDevices()
					Expect(len(devices)).To(Equal(1))
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return an error adding a duplicate device", func() {
					mockDevice.EXPECT().ID().Return(deviceID)
					Expect(client.AddDevice(mockDevice)).To(MatchError(common.ErrDuplicate))
				})

				It("should add another device", func() {
					mockDevice.EXPECT().ID().Return(uint64(deviceUnkownID))
					Expect(client.AddDevice(mockDevice)).To(Succeed())
					devices, _ := client.GetDevices()
					Expect(len(devices)).To(Equal(2))
				})

				It("should add a light", func() {
					mockLight.EXPECT().ID().Return(uint64(lightID))
					Expect(client.AddDevice(mockLight)).To(Succeed())
					devices, _ := client.GetDevices()
					Expect(len(devices)).To(Equal(2))
				})
			})

			Context("finding a device", func() {
				It("should find it by ID", func() {
					dev, err := client.GetDeviceByID(deviceID)
					Expect(dev).To(Equal(mockDevice))
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return an error with the ID is not known", func() {
					_, err := client.GetDeviceByID(deviceUnkownID)
					Expect(err).To(MatchError(common.ErrNotFound))
				})

				It("should find it by label", func() {
					mockDevice.EXPECT().GetLabel().Return(deviceLabel, nil)
					dev, err := client.GetDeviceByLabel(deviceLabel)
					Expect(dev).To(Equal(mockDevice))
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return an error when the label is not known", func() {
					mockDevice.EXPECT().GetLabel().Return(deviceLabel, nil).AnyTimes()
					_, err := client.GetDeviceByLabel(deviceUnkownLabel)
					Expect(err).To(MatchError(common.ErrNotFound))
				})
			})

			Context("removing a device", func() {
				It("should remove it by ID when it is known", func() {
					Expect(client.RemoveDeviceByID(deviceID)).Should(Succeed())
					devices, _ := client.GetDevices()
					Expect(len(devices)).To(Equal(0))
				})

				It("should return an error if the ID is not known", func() {
					Expect(client.RemoveDeviceByID(deviceUnkownID)).To(MatchError(common.ErrNotFound))
					devices, _ := client.GetDevices()
					Expect(len(devices)).To(Equal(1))
				})
			})

			It("should not return any lights", func() {
				lights, err := client.GetLights()
				Expect(len(lights)).To(Equal(0))
				Expect(err).To(MatchError(common.ErrNotFound))
			})

			Context("with lights", func() {
				BeforeEach(func() {
					mockLight.EXPECT().ID().Return(uint64(lightID))
					client.AddDevice(mockLight)
				})

				It("should return only lights", func() {
					lights, err := client.GetLights()
					Expect(len(lights)).To(Equal(1))
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return it by ID when known", func() {
					light, err := client.GetLightByID(lightID)
					Expect(light).To(Equal(mockLight))
					Expect(err).NotTo(HaveOccurred())
				})

				It("should not return a known device by ID if it is not a light", func() {
					light, err := client.GetLightByID(deviceID)
					Expect(light).To(BeNil())
					Expect(err).To(MatchError(common.ErrNotFound))
				})

				It("should return it by label when known", func() {
					mockDevice.EXPECT().GetLabel().Return(deviceLabel, nil).AnyTimes()
					mockLight.EXPECT().GetLabel().Return(lightLabel, nil).AnyTimes()
					light, err := client.GetLightByLabel(lightLabel)
					Expect(light).To(Equal(mockLight))
					Expect(err).NotTo(HaveOccurred())
				})

				It("should not return a known device by label if it is not a light", func() {
					mockDevice.EXPECT().GetLabel().Return(deviceLabel, nil).AnyTimes()
					mockLight.EXPECT().GetLabel().Return(lightLabel, nil).AnyTimes()
					light, err := client.GetLightByLabel(deviceLabel)
					Expect(light).To(BeNil())
					Expect(err).To(MatchError(common.ErrNotFound))
				})

			})

		})

	})

})
