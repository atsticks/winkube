package netutil

import (
	"fmt"
	"gopkg.in/go-playground/assert.v1"
	"testing"
)

//func TestMain(m *testing.M) {
//	// do prepare tasks here...
//	// Run tests
//	os.Exit(m.Run())
//}

func TestCreateCIDR(t *testing.T) {
	cidrs := []string{
		"127.0.0.1/32",
		"192.168.10.0/24",
		"10.2.17.0/16",
	}
	for _, cidr := range cidrs {
		t.Run(cidr, func(t *testing.T) {
			createdCIDR := CreateCIDR(cidr)
			if createdCIDR == nil {
				t.Error("Created CIDR with 127.0.0.1/32 is nil")
			}
		})

	}
}

func TestCIDR_Cidr(t *testing.T) {
	cidrs := []string{
		"127.0.0.1/32",
		"192.168.10.0/24",
		"10.2.17.0/16",
	}
	for _, cidr := range cidrs {
		t.Run(cidr, func(t *testing.T) {
			createdCIDR := CreateCIDR(cidr)
			if (*createdCIDR).Cidr() != cidr {
				t.Error(fmt.Sprintf("CIDR.Cidr() is not %s but %s", cidr, (*createdCIDR).Cidr()))
			}
		})

	}
}

func TestCIDR_UsedIps(t *testing.T) {
	cidrs := []string{
		"127.0.0.1/32",
		"192.168.10.0/24",
		"10.2.17.0/16",
	}
	for _, cidr := range cidrs {
		t.Run(cidr, func(t *testing.T) {
			createdCIDR := CreateCIDR(cidr)
			if (*createdCIDR).UsedIps() == nil {
				t.Error(fmt.Sprintf("CIDR.UsedIps() is nil"))
			}
			if len((*createdCIDR).UsedIps()) != 0 {
				t.Error(fmt.Sprintf("CIDR.UsedIps() is not empty at start."))
			}
		})

	}
}

func TestCIDR_FreeIps(t *testing.T) {
	cidrs := []string{
		"127.0.0.1/32",
		"192.168.10.0/24",
		"10.2.17.0/16",
	}
	for _, cidr := range cidrs {
		t.Run(cidr, func(t *testing.T) {
			createdCIDR := CreateCIDR(cidr)
			if (*createdCIDR).FreeIps() == nil {
				t.Error(fmt.Sprintf("CIDR.FreeIps() is nil"))
			}
			if len((*createdCIDR).FreeIps()) == 0 {
				t.Error(fmt.Sprintf("CIDR.FreeIps() is empty at start."))
			}
		})

	}
}

func TestCIDR_FreeIps_count(t *testing.T) {
	cidrs := []struct {
		cidr         string
		numAddresses int
	}{
		{"127.0.0.1/32", 1},
		{"192.168.10.0/24", 254},
		{"10.2.17.0/16", 65534},
	}

	for _, rec := range cidrs {
		t.Run(rec.cidr, func(t *testing.T) {
			createdCIDR := CreateCIDR(rec.cidr)
			count := len((*createdCIDR).FreeIps())
			if count != rec.numAddresses {
				t.Error(fmt.Sprintf("CIDR.FreeIps() length expected: %v but was %v", rec.numAddresses, count))
			}
		})

	}
}

func TestCIDR_AllIps(t *testing.T) {
	cidrs := []struct {
		cidr         string
		numAddresses int
	}{
		{"127.0.0.1/32", 1},
		{"192.168.10.0/24", 254},
		{"10.2.17.0/16", 65534},
	}

	for _, rec := range cidrs {
		t.Run(rec.cidr, func(t *testing.T) {
			createdCIDR := CreateCIDR(rec.cidr)
			if (*createdCIDR).AllIps() == nil {
				t.Error(fmt.Sprintf("CIDR.AllIps() is nil"))
			}
			count := len((*createdCIDR).AllIps())
			if count != rec.numAddresses {
				t.Error(fmt.Sprintf("CIDR.FreeIps() length expected: %v but was %v", rec.numAddresses, count))
			}
		})

	}
}

func TestCIDR_MarkIpUsed(t *testing.T) {
	cidrs := []struct {
		cidr    string
		markIps []string
	}{
		{"127.0.0.1/32", []string{""}},
		{"192.168.10.0/24", []string{"192.168.10.0", "192.168.10.1"}},
		{"10.2.17.0/16", []string{"10.2.17.10", "10.2.18.13"}},
	}

	for _, rec := range cidrs {
		t.Run(rec.cidr, func(t *testing.T) {
			createdCIDR := CreateCIDR(rec.cidr)
			num := len((*createdCIDR).FreeIps())
			var reserved = 0
			for _, ip := range rec.markIps {
				if (*createdCIDR).MarkIpUsed(ip) {
					reserved++
				}
			}
			numFreeAfter := len((*createdCIDR).FreeIps())
			numUsedAfter := len((*createdCIDR).UsedIps())
			if numUsedAfter != reserved {
				t.Error(fmt.Sprintf("CIDR.MarkIpUsed() length UsedIps() does not match reserved num: %v but expected %v", numUsedAfter, reserved))
			}
			if numFreeAfter != (num - reserved) {
				t.Error(fmt.Sprintf("CIDR.MarkIpUsed() length FreeIps() does not match all - reserved num: %v but expected %v", numFreeAfter, num-reserved))
			}
		})

	}
}

func TestCIDR_MarkAndRelease(t *testing.T) {
	cidrs := []struct {
		cidr       string
		markIps    []string
		releaseIps []string
		usedIp     string
	}{
		{"127.0.0.1/32", []string{""}, []string{""}, ""},
		{"192.168.10.0/24", []string{"192.168.10.0", "192.168.10.1"}, []string{"192.168.10.1"}, ""},
		{"10.2.17.0/16", []string{"10.2.17.10", "10.2.18.13"}, []string{"10.2.17.10"}, "10.2.18.13"},
	}
	for _, rec := range cidrs {
		t.Run(rec.cidr, func(t *testing.T) {
			createdCIDR := CreateCIDR(rec.cidr)
			var reserved = 0
			for _, ip := range rec.markIps {
				if (*createdCIDR).MarkIpUsed(ip) {
					reserved++
				}
			}
			for _, ip := range rec.releaseIps {
				(*createdCIDR).MarkIpUnused(ip)
			}
			numUsedAfter := len((*createdCIDR).UsedIps())
			if rec.usedIp == "" && numUsedAfter > 0 {
				t.Error(fmt.Sprintf("CIDR.Mark & Release: no IP should be used, but was: %v", numUsedAfter))
			}
			if rec.usedIp != "" && numUsedAfter != 1 {
				t.Error(fmt.Sprintf("CIDR.Mark & Release: Only one IP should be in use after release, but was: %v", (*createdCIDR).UsedIps()))
			}
			if rec.usedIp != "" && (*createdCIDR).UsedIps()[0] != rec.usedIp {
				t.Error(fmt.Sprintf("CIDR.Mark & Release: Only this IP should be in use after release %v, but was: %v", rec.usedIp, (*createdCIDR).UsedIps()[0]))
			}
		})

	}
}

func TestCIDR_GetFreeIp(t *testing.T) {
	cidrs := []struct {
		cidr    string
		numNext int
		usedIps []string
	}{
		{"127.0.0.1/32", 2, []string{"127.0.0.1", ""}},
		{"192.168.10.0/24", 2, []string{"192.168.10.1", "192.168.10.2"}},
		{"10.2.0.0/16", 2, []string{"10.2.0.1", "10.2.0.2"}},
	}

	for _, rec := range cidrs {
		t.Run(rec.cidr, func(t *testing.T) {
			createdCIDR := CreateCIDR(rec.cidr)
			var ips = []string{}
			for i := 0; i < rec.numNext; i++ {
				ips = append(ips, (*createdCIDR).GetFreeIp())
			}
			assert.Equal(t, ips, rec.usedIps)
		})
	}
}

func TestCIDR_GetFreeIp_InterceptingReservesAndReleases(t *testing.T) {
	createdCIDR := *CreateCIDR("192.168.10.0/24")
	createdCIDR.MarkIpUsed("192.168.10.1")
	createdCIDR.MarkIpUsed("192.168.10.3")
	createdCIDR.MarkIpUsed("192.168.10.4")
	assert.Equal(t, "192.168.10.2", createdCIDR.GetFreeIp())
	assert.Equal(t, "192.168.10.5", createdCIDR.GetFreeIp())
	createdCIDR.MarkIpUnused("192.168.10.3")
	assert.Equal(t, "192.168.10.3", createdCIDR.GetFreeIp())
	assert.Equal(t, "192.168.10.6", createdCIDR.GetFreeIp())
}
