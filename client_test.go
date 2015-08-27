package golifx_test

import (
	"errors"
	"time"

	. "github.com/pdf/golifx"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	"github.com/pdf/golifx/common"
	"github.com/pdf/golifx/mocks"
	"github.com/stretchr/testify/mock"
)

func init() {
	format.UseStringerRepresentation = false
}

var _ = Describe("Golifx", func() {
	var (
		client               *Client
		protocolSubscription *common.Subscription
		clientSubscription   *common.Subscription
		timeout              = 200 * time.Millisecond

		mockProtocol *mocks.Protocol
		mockDevice   *mocks.Device
		mockLight    *mocks.Light
		mockLocation *mocks.Location
		mockGroup    *mocks.Group

		deviceID           = uint64(1234)
		deviceUnknownID    = uint64(4321)
		deviceLabel        = `mockDevice`
		deviceUnknownLabel = `unknownDevice`
		lightID            = uint64(5678)
		lightLabel         = `mockLight`

		locationID           = `mockLocationID`
		locationUnknownID    = `unknownLocationID`
		locationLabel        = `mockLocation`
		locationUnknownLabel = `unknownLocation`
		groupID              = `mockGroupID`
		groupUnknownID       = `unknownGroupID`
		groupLabel           = `mockGroup`
		groupUnknownLabel    = `unknownGroup`
	)

	It("should send discovery to the protocol on NewClient", func() {
		var err error
		mockProtocol = new(mocks.Protocol)
		mockProtocol.On(`SetClient`, mock.Anything).Return()
		mockProtocol.SubscriptionTarget.On(`NewSubscription`).Return(common.NewSubscription(mockProtocol), nil)
		mockProtocol.On(`Discover`).Return(nil).Once()

		client, err = NewClient(mockProtocol)
		Expect(client).To(BeAssignableToTypeOf(new(Client)))
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Client", func() {
		BeforeEach(func() {
			mockProtocol = new(mocks.Protocol)
			mockProtocol.On(`SetClient`, mock.Anything).Return()
			mockProtocol.SubscriptionTarget.On(`NewSubscription`).Return(common.NewSubscription(mockProtocol), nil)
			mockProtocol.On(`Discover`).Return(nil).Once()
			client, _ = NewClient(mockProtocol)
			client.SetTimeout(timeout)
			protocolSubscription, _ = mockProtocol.NewSubscription()
			clientSubscription, _ = client.NewSubscription()

			mockDevice = new(mocks.Device)
			mockLight = new(mocks.Light)
			mockLocation = new(mocks.Location)
			mockGroup = new(mocks.Group)
		})

		AfterEach(func() {
			mockProtocol.SubscriptionTarget.On(`CloseSubscription`, mock.Anything).Return(nil)
			mockProtocol.On(`Close`).Return(nil)
			_ = client.Close()
		})

		It("should update the timeout", func() {
			t := 5 * time.Second
			client.SetTimeout(t)
			Expect(client.GetTimeout()).To(Equal(&t))
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

		It("should perform discovery on the interval", func() {
			mockProtocol.On(`Discover`).Return(nil).Twice()
			Expect(client.SetDiscoveryInterval(100 * time.Millisecond)).To(Succeed())
			time.Sleep(250 * time.Millisecond)
			mockProtocol.AssertNumberOfCalls(GinkgoT(), `Discover`, 3)
		})

		It("should send SetPower to the protocol", func() {
			mockProtocol.On(`SetPower`, true).Return(nil)
			Expect(client.SetPower(true)).To(Succeed())
		})

		It("should send SetPowerDuration to the protocol", func() {
			duration := 5 * time.Second
			mockProtocol.On(`SetPowerDuration`, true, duration).Return(nil)
			Expect(client.SetPowerDuration(true, duration)).To(Succeed())
		})

		It("should send SetColor to the protocol", func() {
			color := common.Color{}
			duration := 1 * time.Millisecond
			mockProtocol.On(`SetColor`, color, duration).Return(nil)
			Expect(client.SetColor(color, duration)).To(Succeed())
		})

		It("should return an error from GetLocations when it knows no locations", func() {
			locations, err := client.GetLocations()
			Expect(len(locations)).To(Equal(0))
			Expect(err).To(Equal(common.ErrNotFound))
		})

		It("should return an error from GetGroups when it knows no groups", func() {
			groups, err := client.GetGroups()
			Expect(len(groups)).To(Equal(0))
			Expect(err).To(Equal(common.ErrNotFound))
		})

		It("should return an error from GetDevices when it knows no devices", func() {
			devices, err := client.GetDevices()
			Expect(len(devices)).To(Equal(0))
			Expect(err).To(Equal(common.ErrNotFound))
		})

		It("should close successfully", func() {
			mockProtocol.On(`Close`).Return(nil)
			Expect(client.Close()).To(Succeed())
		})

		It("should return an error on failed close", func() {
			mockProtocol.On(`Close`).Return(errors.New(`close failure`))
			Expect(client.Close()).NotTo(Succeed())
		})

		It("should return an error on double-close", func() {
			mockProtocol.On(`Close`).Return(nil)
			Expect(client.Close()).To(Succeed())
			Expect(client.Close()).To(Equal(common.ErrClosed))
		})

		It("should publish an EventNewLocation on discovering a location", func(done Done) {
			mockLocation.Group.On(`ID`).Return(locationID)
			event := common.EventNewLocation{Location: mockLocation}
			ch := make(chan interface{})
			go func() {
				evt := <-clientSubscription.Events()
				ch <- evt
			}()
			_ = protocolSubscription.Write(event)
			Expect(<-ch).To(Equal(event))
			close(done)
		})

		It("should publish an EventNewGroup on discovering a group", func(done Done) {
			mockGroup.On(`ID`).Return(groupID)
			event := common.EventNewGroup{Group: mockGroup}
			ch := make(chan interface{})
			go func() {
				evt := <-clientSubscription.Events()
				ch <- evt
			}()
			_ = protocolSubscription.Write(event)
			Expect(<-ch).To(Equal(event))
			close(done)
		})

		It("should publish an EventNewDevice on discovering a device", func(done Done) {
			mockDevice.On(`ID`).Return(deviceID)
			event := common.EventNewDevice{Device: mockDevice}
			ch := make(chan interface{})
			go func() {
				evt := <-clientSubscription.Events()
				ch <- evt
			}()
			_ = protocolSubscription.Write(event)
			Expect(<-ch).To(Equal(event))
			close(done)
		})

		It("should add a location", func(done Done) {
			mockLocation.Group.On(`ID`).Return(locationID)
			ch := make(chan bool)
			go func() {
				<-clientSubscription.Events()
				ch <- true
			}()
			_ = protocolSubscription.Write(common.EventNewLocation{Location: mockLocation})
			<-ch
			locations, err := client.GetLocations()
			Expect(len(locations)).To(Equal(1))
			Expect(err).NotTo(HaveOccurred())
			close(done)
		})

		It("should add a group", func(done Done) {
			mockGroup.On(`ID`).Return(groupID)
			ch := make(chan bool)
			go func() {
				<-clientSubscription.Events()
				ch <- true
			}()
			_ = protocolSubscription.Write(common.EventNewGroup{Group: mockGroup})
			<-ch
			groups, err := client.GetGroups()
			Expect(len(groups)).To(Equal(1))
			Expect(err).NotTo(HaveOccurred())
			close(done)
		})

		It("should add a device", func(done Done) {
			mockDevice.On(`ID`).Return(deviceID)
			ch := make(chan bool)
			go func() {
				<-clientSubscription.Events()
				ch <- true
			}()
			_ = protocolSubscription.Write(common.EventNewDevice{Device: mockDevice})
			<-ch
			devices, err := client.GetDevices()
			Expect(len(devices)).To(Equal(1))
			Expect(err).NotTo(HaveOccurred())
			close(done)
		})

		Context("with locations", func() {

			BeforeEach(func() {
				mockLocation.Group.On(`ID`).Return(locationID).Once()
				clientSubscription, _ = client.NewSubscription()
				_ = protocolSubscription.Write(common.EventNewLocation{Location: mockLocation})
				<-clientSubscription.Events()
			})

			Context("adding a location", func() {
				It("should return the location", func() {
					locations, err := client.GetLocations()
					Expect(len(locations)).To(Equal(1))
					Expect(locations[0]).To(Equal(mockLocation))
					Expect(err).NotTo(HaveOccurred())
				})

				It("should not add a duplicate location", func(done Done) {
					var err error
					Expect(err).NotTo(HaveOccurred())
					ch := make(chan bool)
					go func() {
						<-clientSubscription.Events()
						ch <- false
					}()
					time.AfterFunc(timeout*2, func() {
						ch <- true
					})
					mockLocation.Group.On(`ID`).Return(locationID).Once()
					_ = protocolSubscription.Write(common.EventNewLocation{Location: mockLocation})
					Expect(<-ch).To(Equal(true))
					locations, err := client.GetLocations()
					Expect(len(locations)).To(Equal(1))
					Expect(err).NotTo(HaveOccurred())
					close(done)
				})

				It("should add another location", func(done Done) {
					mockLocation.Group.On(`ID`).Return(locationUnknownID).Once()
					ch := make(chan bool)
					go func() {
						<-clientSubscription.Events()
						ch <- true
					}()
					_ = protocolSubscription.Write(common.EventNewLocation{Location: mockLocation})
					<-ch
					locations, _ := client.GetLocations()
					Expect(len(locations)).To(Equal(2))
					close(done)
				})
			})

			Context("finding a location", func() {
				It("should find it by ID", func() {
					loc, err := client.GetLocationByID(locationID)
					Expect(loc).To(Equal(mockLocation))
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return an error when the ID is not known", func() {
					_, err := client.GetLocationByID(locationUnknownID)
					Expect(err).To(MatchError(common.ErrNotFound))
				})

				It("should find it by label", func() {
					mockLocation.Group.On(`GetLabel`).Return(locationLabel, nil).Once()
					loc, err := client.GetLocationByLabel(locationLabel)
					Expect(loc).To(Equal(mockLocation))
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return an error when the label is not known", func() {
					mockLocation.Group.On(`GetLabel`).Return(locationLabel, nil)
					_, err := client.GetLocationByLabel(locationUnknownLabel)
					Expect(err).To(MatchError(common.ErrNotFound))
				})

				Context("when the location is added while searching", func() {

					It("should find it by ID", func(done Done) {
						locChan := make(chan common.Location)
						errChan := make(chan error)
						unknownLocation := new(mocks.Location)
						go func() {
							loc, err := client.GetLocationByID(locationUnknownID)
							errChan <- err
							locChan <- loc
						}()
						unknownLocation.Group.On(`ID`).Return(locationUnknownID).Once()
						_ = protocolSubscription.Write(common.EventNewLocation{Location: unknownLocation})
						Expect(<-errChan).NotTo(HaveOccurred())
						Expect(<-locChan).To(Equal(unknownLocation))
						close(done)
					})

					It("should find it by label", func(done Done) {
						locChan := make(chan common.Location)
						errChan := make(chan error)
						unknownLocation := new(mocks.Location)
						mockLocation.Group.On(`GetLabel`).Return(locationLabel, nil).Once()
						go func() {
							loc, err := client.GetLocationByLabel(locationUnknownLabel)
							errChan <- err
							locChan <- loc
						}()
						unknownLocation.Group.On(`ID`).Return(locationUnknownID).Once()
						unknownLocation.Group.On(`GetLabel`).Return(locationUnknownLabel, nil).Once()
						_ = protocolSubscription.Write(common.EventNewLocation{Location: unknownLocation})
						Expect(<-errChan).NotTo(HaveOccurred())
						Expect(<-locChan).To(Equal(unknownLocation))
						close(done)
					})

				})

				Context("with zero timeout", func() {
					BeforeEach(func() {
						client.SetTimeout(0)
					})

					It("should not timeout searching by ID", func(done Done) {
						time.AfterFunc(10*time.Millisecond, func() {
							close(done)
						})

						_, err := client.GetLocationByID(locationUnknownID)
						Expect(err).NotTo(HaveOccurred())
					})

					It("should not timeout searching by label", func(done Done) {
						time.AfterFunc(10*time.Millisecond, func() {
							close(done)
						})

						mockLocation.Group.On(`GetLabel`).Return(locationLabel, nil)
						_, err := client.GetLocationByLabel(locationUnknownLabel)
						Expect(err).NotTo(HaveOccurred())
					})
				})
			})

			Context("removing a location", func() {
				It("should emit an EventExpiredLocation when a location is removed", func(done Done) {
					mockLocation.Group.On(`ID`).Return(locationID).Once()
					event := common.EventExpiredLocation{Location: mockLocation}
					ch := make(chan interface{})
					go func() {
						evt := <-clientSubscription.Events()
						ch <- evt
					}()
					_ = protocolSubscription.Write(event)
					Expect(<-ch).To(Equal(event))
					close(done)
				})

				It("should not remove it when it is not known", func(done Done) {
					mockLocation.Group.On(`ID`).Return(locationUnknownID).Once()
					event := common.EventExpiredLocation{Location: mockLocation}
					ch := make(chan bool)
					go func() {
						<-clientSubscription.Events()
						ch <- false
					}()
					time.AfterFunc(timeout*2, func() {
						ch <- true
					})
					_ = protocolSubscription.Write(event)
					Expect(<-ch).To(Equal(true))
					locations, _ := client.GetLocations()
					Expect(len(locations)).To(Equal(1))
					close(done)
				})

			})
		})

		Context("with groups", func() {

			BeforeEach(func() {
				mockGroup.On(`ID`).Return(groupID).Once()
				clientSubscription, _ = client.NewSubscription()
				_ = protocolSubscription.Write(common.EventNewGroup{Group: mockGroup})
				<-clientSubscription.Events()
			})

			Context("adding a group", func() {
				It("should return the group", func() {
					groups, err := client.GetGroups()
					Expect(len(groups)).To(Equal(1))
					Expect(groups[0]).To(Equal(mockGroup))
					Expect(err).NotTo(HaveOccurred())
				})

				It("should not add a duplicate group", func(done Done) {
					var err error
					Expect(err).NotTo(HaveOccurred())
					ch := make(chan bool)
					go func() {
						<-clientSubscription.Events()
						ch <- false
					}()
					time.AfterFunc(timeout*2, func() {
						ch <- true
					})
					mockGroup.On(`ID`).Return(groupID).Once()
					_ = protocolSubscription.Write(common.EventNewGroup{Group: mockGroup})
					Expect(<-ch).To(Equal(true))
					groups, err := client.GetGroups()
					Expect(len(groups)).To(Equal(1))
					Expect(err).NotTo(HaveOccurred())
					close(done)
				})

				It("should add another group", func(done Done) {
					mockGroup.On(`ID`).Return(groupUnknownID).Once()
					ch := make(chan bool)
					go func() {
						<-clientSubscription.Events()
						ch <- true
					}()
					_ = protocolSubscription.Write(common.EventNewGroup{Group: mockGroup})
					<-ch
					groups, _ := client.GetGroups()
					Expect(len(groups)).To(Equal(2))
					close(done)
				})
			})

			Context("finding a group", func() {
				It("should find it by ID", func() {
					grp, err := client.GetGroupByID(groupID)
					Expect(grp).To(Equal(mockGroup))
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return an error when the ID is not known", func() {
					_, err := client.GetGroupByID(groupUnknownID)
					Expect(err).To(MatchError(common.ErrNotFound))
				})

				It("should find it by label", func() {
					mockGroup.On(`GetLabel`).Return(groupLabel, nil).Once()
					grp, err := client.GetGroupByLabel(groupLabel)
					Expect(grp).To(Equal(mockGroup))
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return an error when the label is not known", func() {
					mockGroup.On(`GetLabel`).Return(groupLabel, nil)
					_, err := client.GetGroupByLabel(groupUnknownLabel)
					Expect(err).To(MatchError(common.ErrNotFound))
				})

				Context("when the group is added while searching", func() {

					It("should find it by ID", func(done Done) {
						grpChan := make(chan common.Group)
						errChan := make(chan error)
						unknownGroup := new(mocks.Group)
						go func() {
							grp, err := client.GetGroupByID(groupUnknownID)
							errChan <- err
							grpChan <- grp
						}()
						unknownGroup.On(`ID`).Return(groupUnknownID).Once()
						_ = protocolSubscription.Write(common.EventNewGroup{Group: unknownGroup})
						Expect(<-errChan).NotTo(HaveOccurred())
						Expect(<-grpChan).To(Equal(unknownGroup))
						close(done)
					})

					It("should find it by label", func(done Done) {
						grpChan := make(chan common.Group)
						errChan := make(chan error)
						unknownGroup := new(mocks.Group)
						mockGroup.On(`GetLabel`).Return(groupLabel, nil).Once()
						go func() {
							grp, err := client.GetGroupByLabel(groupUnknownLabel)
							errChan <- err
							grpChan <- grp
						}()
						unknownGroup.On(`ID`).Return(groupUnknownID).Once()
						unknownGroup.On(`GetLabel`).Return(groupUnknownLabel, nil).Once()
						_ = protocolSubscription.Write(common.EventNewGroup{Group: unknownGroup})
						Expect(<-errChan).NotTo(HaveOccurred())
						Expect(<-grpChan).To(Equal(unknownGroup))
						close(done)
					})

				})

				Context("with zero timeout", func() {
					BeforeEach(func() {
						client.SetTimeout(0)
					})

					It("should not timeout searching by ID", func(done Done) {
						time.AfterFunc(10*time.Millisecond, func() {
							close(done)
						})

						_, err := client.GetGroupByID(groupUnknownID)
						Expect(err).NotTo(HaveOccurred())
					})

					It("should not timeout searching by label", func(done Done) {
						time.AfterFunc(10*time.Millisecond, func() {
							close(done)
						})

						mockGroup.On(`GetLabel`).Return(groupLabel, nil)
						_, err := client.GetGroupByLabel(groupUnknownLabel)
						Expect(err).NotTo(HaveOccurred())
					})
				})
			})

			Context("removing a group", func() {
				It("should emit an EventExpiredGroup when a group is removed", func(done Done) {
					mockGroup.On(`ID`).Return(groupID).Once()
					event := common.EventExpiredGroup{Group: mockGroup}
					ch := make(chan interface{})
					go func() {
						evt := <-clientSubscription.Events()
						ch <- evt
					}()
					_ = protocolSubscription.Write(event)
					Expect(<-ch).To(Equal(event))
					close(done)
				})

				It("should not remove it when it is not known", func(done Done) {
					mockGroup.On(`ID`).Return(groupUnknownID).Once()
					event := common.EventExpiredGroup{Group: mockGroup}
					ch := make(chan bool)
					go func() {
						<-clientSubscription.Events()
						ch <- false
					}()
					time.AfterFunc(timeout*2, func() {
						ch <- true
					})
					_ = protocolSubscription.Write(event)
					Expect(<-ch).To(Equal(true))
					groups, _ := client.GetGroups()
					Expect(len(groups)).To(Equal(1))
					close(done)
				})

			})
		})

		Context("with devices", func() {

			BeforeEach(func() {
				mockDevice.On(`ID`).Return(deviceID).Once()
				clientSubscription, _ = client.NewSubscription()
				_ = protocolSubscription.Write(common.EventNewDevice{Device: mockDevice})
				<-clientSubscription.Events()
			})

			Context("adding a device", func() {
				It("should return the device", func() {
					devices, err := client.GetDevices()
					Expect(len(devices)).To(Equal(1))
					Expect(devices[0]).To(Equal(mockDevice))
					Expect(err).NotTo(HaveOccurred())
				})

				It("should not add a duplicate device", func(done Done) {
					var err error
					Expect(err).NotTo(HaveOccurred())
					ch := make(chan bool)
					go func() {
						<-clientSubscription.Events()
						ch <- false
					}()
					time.AfterFunc(timeout*2, func() {
						ch <- true
					})
					mockDevice.On(`ID`).Return(deviceID).Once()
					_ = protocolSubscription.Write(common.EventNewDevice{Device: mockDevice})
					Expect(<-ch).To(Equal(true))
					devices, err := client.GetDevices()
					Expect(len(devices)).To(Equal(1))
					Expect(err).NotTo(HaveOccurred())
					close(done)
				})

				It("should add another device", func(done Done) {
					mockDevice.On(`ID`).Return(deviceUnknownID).Once()
					ch := make(chan bool)
					go func() {
						<-clientSubscription.Events()
						ch <- true
					}()
					_ = protocolSubscription.Write(common.EventNewDevice{Device: mockDevice})
					<-ch
					devices, _ := client.GetDevices()
					Expect(len(devices)).To(Equal(2))
					close(done)
				})

				It("should add a light", func(done Done) {
					mockLight.Device.On(`ID`).Return(lightID).Once()
					ch := make(chan bool)
					go func() {
						<-clientSubscription.Events()
						ch <- true
					}()
					_ = protocolSubscription.Write(common.EventNewDevice{Device: mockLight})
					<-ch
					devices, _ := client.GetDevices()
					Expect(len(devices)).To(Equal(2))
					close(done)
				})

			})

			Context("finding a device", func() {
				It("should find it by ID", func() {
					dev, err := client.GetDeviceByID(deviceID)
					Expect(dev).To(Equal(mockDevice))
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return an error when the ID is not known", func() {
					_, err := client.GetDeviceByID(deviceUnknownID)
					Expect(err).To(MatchError(common.ErrNotFound))
				})

				It("should find it by label", func() {
					mockDevice.On(`GetLabel`).Return(deviceLabel, nil).Once()
					dev, err := client.GetDeviceByLabel(deviceLabel)
					Expect(dev).To(Equal(mockDevice))
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return an error when the label is not known", func() {
					mockDevice.On(`GetLabel`).Return(deviceLabel, nil)
					_, err := client.GetDeviceByLabel(deviceUnknownLabel)
					Expect(err).To(MatchError(common.ErrNotFound))
				})

				Context("when the device is added while searching", func() {

					It("should find it by ID", func(done Done) {
						devChan := make(chan common.Device)
						errChan := make(chan error)
						unknownDevice := new(mocks.Device)
						go func() {
							dev, err := client.GetDeviceByID(deviceUnknownID)
							errChan <- err
							devChan <- dev
						}()
						unknownDevice.On(`ID`).Return(deviceUnknownID).Once()
						_ = protocolSubscription.Write(common.EventNewDevice{Device: unknownDevice})
						Expect(<-errChan).NotTo(HaveOccurred())
						Expect(<-devChan).To(Equal(unknownDevice))
						close(done)
					})

					It("should find it by label", func(done Done) {
						devChan := make(chan common.Device)
						errChan := make(chan error)
						unknownDevice := new(mocks.Device)
						mockDevice.On(`GetLabel`).Return(deviceLabel, nil).Once()
						go func() {
							dev, err := client.GetDeviceByLabel(deviceUnknownLabel)
							errChan <- err
							devChan <- dev
						}()
						unknownDevice.On(`ID`).Return(deviceUnknownID).Once()
						unknownDevice.On(`GetLabel`).Return(deviceUnknownLabel, nil).Once()
						_ = protocolSubscription.Write(common.EventNewDevice{Device: unknownDevice})
						Expect(<-errChan).NotTo(HaveOccurred())
						Expect(<-devChan).To(Equal(unknownDevice))
						close(done)
					})

				})

				Context("with zero timeout", func() {
					BeforeEach(func() {
						client.SetTimeout(0)
					})

					It("should not timeout searching by ID", func(done Done) {
						time.AfterFunc(10*time.Millisecond, func() {
							close(done)
						})

						_, err := client.GetDeviceByID(deviceUnknownID)
						Expect(err).NotTo(HaveOccurred())
					})

					It("should not timeout searching by label", func(done Done) {
						time.AfterFunc(10*time.Millisecond, func() {
							close(done)
						})

						mockDevice.On(`GetLabel`).Return(deviceLabel, nil)
						_, err := client.GetDeviceByLabel(deviceUnknownLabel)
						Expect(err).NotTo(HaveOccurred())
					})
				})
			})

			Context("removing a device", func() {
				It("should emit an EventExpiredDevice when a device is removed", func(done Done) {
					mockDevice.On(`ID`).Return(deviceID).Once()
					event := common.EventExpiredDevice{Device: mockDevice}
					ch := make(chan interface{})
					go func() {
						evt := <-clientSubscription.Events()
						ch <- evt
					}()
					_ = protocolSubscription.Write(event)
					Expect(<-ch).To(Equal(event))
					close(done)
				})

				It("should not remove it when it is not known", func(done Done) {
					mockDevice.On(`ID`).Return(deviceUnknownID).Once()
					event := common.EventExpiredDevice{Device: mockDevice}
					ch := make(chan bool)
					go func() {
						<-clientSubscription.Events()
						ch <- false
					}()
					time.AfterFunc(timeout*2, func() {
						ch <- true
					})
					_ = protocolSubscription.Write(event)
					Expect(<-ch).To(Equal(true))
					devices, _ := client.GetDevices()
					Expect(len(devices)).To(Equal(1))
					close(done)
				})

			})

			It("should not return any lights", func() {
				lights, err := client.GetLights()
				Expect(len(lights)).To(Equal(0))
				Expect(err).To(MatchError(common.ErrNotFound))
			})

			Context("with lights", func() {
				BeforeEach(func() {
					mockLight.Device.On(`ID`).Return(lightID).Once()
					clientSubscription, _ = client.NewSubscription()
					_ = protocolSubscription.Write(common.EventNewDevice{Device: mockLight})
					<-clientSubscription.Events()
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
					Expect(err).To(HaveOccurred())
				})

				It("should return it by label when known", func() {
					mockDevice.On(`GetLabel`).Return(deviceLabel, nil)
					mockLight.Device.On(`GetLabel`).Return(lightLabel, nil)
					light, err := client.GetLightByLabel(lightLabel)
					Expect(light).To(Equal(mockLight))
					Expect(err).NotTo(HaveOccurred())
				})

				It("should not return a known device by label if it is not a light", func() {
					mockDevice.On(`GetLabel`).Return(deviceLabel, nil)
					mockLight.Device.On(`GetLabel`).Return(lightLabel, nil)
					light, err := client.GetLightByLabel(deviceLabel)
					Expect(light).To(BeNil())
					Expect(err).To(MatchError(common.ErrDeviceInvalidType))
				})

				Context("with zero timeout", func() {
					BeforeEach(func() {
						client.SetTimeout(0)
					})

					It("should not timeout searching by ID", func(done Done) {
						time.AfterFunc(10*time.Millisecond, func() {
							close(done)
						})

						_, err := client.GetLightByID(deviceUnknownID)
						Expect(err).NotTo(HaveOccurred())
					})

					It("should not timeout searching by label", func(done Done) {
						time.AfterFunc(10*time.Millisecond, func() {
							close(done)
						})

						mockDevice.On(`GetLabel`).Return(deviceLabel, nil)
						mockLight.Device.On(`GetLabel`).Return(lightLabel, nil)
						_, err := client.GetLightByLabel(deviceUnknownLabel)
						Expect(err).NotTo(HaveOccurred())
					})
				})

			})

		})

	})

})
