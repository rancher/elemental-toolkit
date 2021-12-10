package mocks

type FakeSyscall struct{
	chrootHistory []string  // Track calls to chroot
}

func (f *FakeSyscall) Chroot(path string)  error {
	f.chrootHistory = append(f.chrootHistory, path)
	return nil	
}

func (f *FakeSyscall) WasChrootCalledWith(path string) bool {
	for _, c := range f.chrootHistory {
		if c == path {
			return true
		}
	}
	return false
}
