import React, { useState, useCallback } from 'react';
import {
  View,
  StyleSheet,
  ScrollView,
  Text,
  TextInput,
  TouchableOpacity,
  Alert,
  KeyboardAvoidingView,
  Platform,
} from 'react-native';
import { Camera, MapPin, FileText, User, Phone, PenLine } from 'lucide-react-native';
import { colors, spacing } from '../../utils/theme';
import Button from '../ui/Button';
import Input from '../ui/Input';
import Card from '../ui/Card';
import SignaturePad from './SignaturePad';
import PhotoCapture from './PhotoCapture';
import LocationVerification, { Coordinates } from './LocationVerification';

interface PODInput {
  recipientName: string;
  recipientPhone?: string;
  signatureBase64: string;
  photos: string[];
  notes: string;
  locationVerified: boolean;
  locationMismatchMeters?: number;
  mismatchReason?: string;
}

interface PODFormProps {
  shipmentId: string;
  destinationCoords?: Coordinates;
  onSubmit: (pod: PODInput) => void;
}

const QUICK_NOTES = [
  'Left with security',
  'Door delivery',
  'Partial delivery',
  'Left with neighbor',
  'Customer unavailable',
  'Delivered to reception',
];

const PODForm: React.FC<PODFormProps> = ({
  shipmentId,
  destinationCoords,
  onSubmit,
}) => {
  const [recipientName, setRecipientName] = useState('');
  const [recipientPhone, setRecipientPhone] = useState('');
  const [signatureBase64, setSignatureBase64] = useState<string | null>(null);
  const [photos, setPhotos] = useState<string[]>([]);
  const [notes, setNotes] = useState('');
  const [locationVerified, setLocationVerified] = useState(false);
  const [locationMismatchMeters, setLocationMismatchMeters] = useState<number | undefined>();
  const [mismatchReason, setMismatchReason] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleSignatureChange = useCallback((base64: string | null) => {
    setSignatureBase64(base64);
  }, []);

  const handleAddPhoto = useCallback(() => {
    if (photos.length >= 3) {
      Alert.alert('Limit Reached', 'Maximum 3 photos allowed');
      return;
    }
    // In real implementation, this would open camera
    // For now, add a placeholder
    const placeholderUri = `photo_${Date.now()}`;
    setPhotos((prev) => [...prev, placeholderUri]);
  }, [photos.length]);

  const handleRemovePhoto = useCallback((index: number) => {
    setPhotos((prev) => prev.filter((_, i) => i !== index));
  }, []);

  const handleLocationVerified = useCallback(
    (verified: boolean, mismatchMeters?: number) => {
      setLocationVerified(verified);
      setLocationMismatchMeters(mismatchMeters);
    },
    []
  );

  const handleQuickNote = useCallback((note: string) => {
    setNotes((prev) => {
      if (prev.includes(note)) {
        return prev.replace(`${note}, `, '').replace(note, '').trim();
      }
      return prev ? `${prev}, ${note}` : note;
    });
  }, []);

  const validateEthiopianPhone = (phone: string): boolean => {
    if (!phone) return true;
    const cleaned = phone.replace(/[\s-]/g, '');
    const ethiopianPhoneRegex = /^(\+251|0)[0-9]{9}$/;
    return ethiopianPhoneRegex.test(cleaned);
  };

  const handleSubmit = async () => {
    if (!recipientName.trim()) {
      Alert.alert('Required', 'Please enter the recipient name');
      return;
    }

    if (recipientPhone && !validateEthiopianPhone(recipientPhone)) {
      Alert.alert('Invalid Phone', 'Please enter a valid Ethiopian phone number');
      return;
    }

    if (!signatureBase64) {
      Alert.alert('Required', 'Please capture the recipient signature');
      return;
    }

    setIsSubmitting(true);

    try {
      await onSubmit({
        recipientName: recipientName.trim(),
        recipientPhone: recipientPhone.trim() || undefined,
        signatureBase64,
        photos,
        notes: notes.trim(),
        locationVerified,
        locationMismatchMeters,
        mismatchReason: mismatchReason.trim() || undefined,
      });
    } catch (error) {
      Alert.alert('Error', 'Failed to submit proof of delivery');
    } finally {
      setIsSubmitting(false);
    }
  };

  const isFormValid = recipientName.trim().length > 0 && signatureBase64 !== null;

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === 'ios' ? 'padding' : undefined}
    >
      <ScrollView
        style={styles.scrollView}
        contentContainerStyle={styles.scrollContent}
        showsVerticalScrollIndicator={false}
        keyboardShouldPersistTaps="handled"
      >
        {destinationCoords && (
          <Card style={styles.section}>
            <View style={styles.sectionHeader}>
              <MapPin size={20} color={colors.primary} />
              <Text style={styles.sectionTitle}>Location Verification</Text>
            </View>
            <LocationVerification
              destinationCoords={destinationCoords}
              onVerified={handleLocationVerified}
            />
          </Card>
        )}

        <Card style={styles.section}>
          <View style={styles.sectionHeader}>
            <User size={20} color={colors.primary} />
            <Text style={styles.sectionTitle}>Recipient Information</Text>
          </View>
          
          <Input
            label="Recipient Name *"
            placeholder="Enter recipient's full name"
            value={recipientName}
            onChangeText={setRecipientName}
            autoCapitalize="words"
            leftIcon={<User size={18} color={colors.textSecondary} />}
          />
          
          <Input
            label="Recipient Phone (Optional)"
            placeholder="e.g., +251 9XX XXX XXX"
            value={recipientPhone}
            onChangeText={setRecipientPhone}
            keyboardType="phone-pad"
            leftIcon={<Phone size={18} color={colors.textSecondary} />}
          />
        </Card>

        <Card style={styles.section}>
          <View style={styles.sectionHeader}>
            <PenLine size={20} color={colors.primary} />
            <Text style={styles.sectionTitle}>Signature *</Text>
          </View>
          <SignaturePad onSignatureChange={handleSignatureChange} />
        </Card>

        <Card style={styles.section}>
          <View style={styles.sectionHeader}>
            <Camera size={20} color={colors.primary} />
            <Text style={styles.sectionTitle}>Photos</Text>
          </View>
          <PhotoCapture
            photos={photos}
            onAddPhoto={handleAddPhoto}
            onRemovePhoto={handleRemovePhoto}
            maxPhotos={3}
          />
        </Card>

        <Card style={styles.section}>
          <View style={styles.sectionHeader}>
            <FileText size={20} color={colors.primary} />
            <Text style={styles.sectionTitle}>Delivery Notes</Text>
          </View>
          
          <View style={styles.quickNotesContainer}>
            <Text style={styles.quickNotesLabel}>Quick Select:</Text>
            <View style={styles.quickNotesGrid}>
              {QUICK_NOTES.map((note) => (
                <TouchableOpacity
                  key={note}
                  style={[
                    styles.quickNoteChip,
                    notes.includes(note) && styles.quickNoteChipActive,
                  ]}
                  onPress={() => handleQuickNote(note)}
                >
                  <Text
                    style={[
                      styles.quickNoteText,
                      notes.includes(note) && styles.quickNoteTextActive,
                    ]}
                  >
                    {note}
                  </Text>
                </TouchableOpacity>
              ))}
            </View>
          </View>
          
          <View style={styles.notesInputContainer}>
            <TextInput
              style={styles.notesInput}
              placeholder="Add additional delivery notes..."
              value={notes}
              onChangeText={setNotes}
              multiline
              numberOfLines={4}
              placeholderTextColor={colors.textSecondary}
              textAlignVertical="top"
            />
          </View>
        </Card>

        <View style={styles.submitContainer}>
          <Button
            title="Submit Delivery"
            onPress={handleSubmit}
            loading={isSubmitting}
            disabled={!isFormValid || isSubmitting}
            fullWidth
            size="lg"
          />
          {!isFormValid && (
            <Text style={styles.hint}>
              {!recipientName.trim()
                ? 'Enter recipient name'
                : 'Capture signature to submit'}
            </Text>
          )}
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: colors.background,
  },
  scrollView: {
    flex: 1,
  },
  scrollContent: {
    padding: spacing.lg,
    paddingBottom: spacing.xxl,
  },
  section: {
    marginBottom: spacing.lg,
  },
  sectionHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: spacing.lg,
    gap: spacing.sm,
  },
  sectionTitle: {
    fontSize: 18,
    fontWeight: '600',
    color: colors.text,
  },
  quickNotesContainer: {
    marginBottom: spacing.md,
  },
  quickNotesLabel: {
    fontSize: 14,
    fontWeight: '500',
    color: colors.textSecondary,
    marginBottom: spacing.sm,
  },
  quickNotesGrid: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.sm,
  },
  quickNoteChip: {
    paddingVertical: spacing.xs,
    paddingHorizontal: spacing.md,
    borderRadius: 16,
    backgroundColor: colors.background,
    borderWidth: 1,
    borderColor: colors.border,
  },
  quickNoteChipActive: {
    backgroundColor: colors.primary,
    borderColor: colors.primary,
  },
  quickNoteText: {
    fontSize: 13,
    color: colors.textSecondary,
  },
  quickNoteTextActive: {
    color: colors.surface,
    fontWeight: '500',
  },
  notesInputContainer: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    backgroundColor: colors.surface,
  },
  notesInput: {
    padding: spacing.md,
    fontSize: 16,
    color: colors.text,
    minHeight: 100,
  },
  submitContainer: {
    marginTop: spacing.md,
  },
  hint: {
    fontSize: 12,
    color: colors.textSecondary,
    textAlign: 'center',
    marginTop: spacing.sm,
  },
});

export default PODForm;
export { PODFormProps, PODInput };
