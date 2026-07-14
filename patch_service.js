const fs = require('fs');
const path = '/Users/sandarbh/Desktop/Study/channel-manager/services/integration/usecases/service.go';
let code = fs.readFileSync(path, 'utf8');

const target = `func (s *Service) loadProperty(ctx context.Context, propertyID string) (struct {
	ID, Name, DefaultCurrency string
}, error) {
	prop, err := s.props.GetByID(ctx, propertyID)
	if err != nil {
		return struct{ ID, Name, DefaultCurrency string }{}, fmt.Errorf("property not found: %w", err)
	}`;

const replacement = `func (s *Service) loadProperty(ctx context.Context, propertyID string) (struct {
	ID, Name, DefaultCurrency string
}, error) {
	prop, err := s.props.GetByID(ctx, propertyID)
	if err != nil {
		// Fallback to searching by ExternalID (which is the PMS property ID)
		prop, err = s.props.GetByExternalID(ctx, "", propertyID)
		if err != nil {
			return struct{ ID, Name, DefaultCurrency string }{}, fmt.Errorf("property not found by ID or ExternalID: %w", err)
		}
	}`;

code = code.replace(target, replacement);
fs.writeFileSync(path, code);
console.log('Patched loadProperty');
