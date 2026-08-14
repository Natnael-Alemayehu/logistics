import React, { useState, useCallback, useRef } from 'react';
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
  ActivityIndicator,
} from 'react-native';
import { Camera, MapPin, FileText, User, Phone, PenLine, WifiOff } from 'lucide-react-native';
import { colors, spacing } from '../../utils/theme';
import Button from '../ui/Button';
import Input from '../ui/Input';
import Card from '../ui/Card';
import SignaturePad, { SignaturePadRef } from './SignaturePad';
import PhotoCapture from './PhotoCapture';
import LocationVerification from './LocationVerification';
import { PODPhoto, Coordinates } from '../../types/pod';
import { useTranslation } from 'react-i18next';

export interface PODFormData {
  recipientName: string;
  recipientPhone?: string;
  signatureBase64: string;
  photos: PODPhoto[];
  notes: string;
  locationVerified: boolean;
  locationMismatchMeters?: number;
  mismatchReason?: string;
}

interface PODFormProps {
  shipmentId: string;
  destinationCoords?: Coordinates;
  onSubmit: (pod: PODFormData) => Promise<void> | void;
  isOffline?: boolean;
}

const PODForm: React.FC<PODFormProps> = ({
  shipmentId,
  destinationCoords,
  onSubmit,
  isOffline = false,
}) => {
  const { t } = useTranslation();
  const [recipientName, setRecipientName] = useState('');
  const [recipientPhone, setRecipientPhone] = useState('');
  const [signatureBase64, setSignatureBase64] = useState<string | null>(null);
  const [photos, setPhotos] = useState<PODPhoto[]>([]);
  const [notes, setNotes] = useState('');
  const [locationVerified, setLocationVerified] = useState(false);
  const [locationMismatchMeters, setLocationMismatchMeters] = useState<number | undefined>();
  const [mismatchReason, setMismatchReason] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  
  const signaturePadRef = useRef<SignaturePadRef>(null);

  const QUICK_NOTES = [
    t('pod.quickNotes.leftWithSecurity'),
    t('pod.quickNotes.doorDelivery'),
    t('pod.quickNotes.partialDelivery'),
    t('pod.quickNotes.leftWithNeighbor'),
    t('pod.quickNotes.customerUnavailable'),
    t('pod.quickNotes.deliveredToReception'),
  ];

  const handleSignatureChange = useCallback((base64: string | null) => {
    setSignatureBase64(base64);
  }, []);

  const handleAddPhoto = useCallback((photo: PODPhoto) => {
    if (photos.length >= 3) {
      Alert.alert(t('pod.limitReached'), t('pod.maxPhotosAllowed'));
      return;
    }
    setPhotos((prev) => [...prev, photo]);
  }, [photos.length, t]);

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
    setSubmitError(null);
    
    if (!recipientName.trim()) {
      setSubmitError(t('pod.recipientNameRequired'));
      return;
    }

    if (recipientPhone && !validateEthiopianPhone(recipientPhone)) {
      setSubmitError(t('pod.invalidPhone'));
      return;
    }

    if (!signatureBase64) {
      setSubmitError(t('pod.signatureRequired'));
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
      setSubmitError(t('pod.submitError'));
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
              <Text style={styles.sectionTitle}>{t('pod.locationVerification')}</Text>
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
            <Text style={styles.sectionTitle}>{t('pod.recipientInfo')}</Text>
          </View>
          
          <Input
            label={`${t('pod.recipientName')} *`}
            placeholder={t('pod.recipientNamePlaceholder')}
            value={recipientName}
            onChangeText={setRecipientName}
            autoCapitalize="words"
            leftIcon={<User size={18} color={colors.textSecondary} />}
          />
          
          <Input
            label={`${t('pod.recipientPhone')} (${t('common.optional')})`}
            placeholder={t('pod.recipientPhonePlaceholder')}
            value={recipientPhone}
            onChangeText={setRecipientPhone}
            keyboardType="phone-pad"
            leftIcon={<Phone size={18} color={colors.textSecondary} />}
          />
        </Card>

        <Card style={styles.section}>
          <View style={styles.sectionHeader}>
            <PenLine size={20} color={colors.primary} />
            <Text style={styles.sectionTitle}>{t('pod.signature')} *</Text>
          </View>
          <SignaturePad 
            ref={signaturePadRef}
            onSignatureChange={handleSignatureChange} 
          />
        </Card>

        <Card style={styles.section}>
          <View style={styles.sectionHeader}>
            <Camera size={20} color={colors.primary} />
            <Text style={styles.sectionTitle}>{t('pod.photos')}</Text>
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
            <Text style={styles.sectionTitle}>{t('pod.deliveryNotes')}</Text>
          </View>
          
          <View style={styles.quickNotesContainer}>
            <Text style={styles.quickNotesLabel}>{t('pod.quickSelect')}:</Text>
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
              placeholder={t('pod.notesPlaceholder')}
              value={notes}
              onChangeText={setNotes}
              multiline
              numberOfLines={4}
              placeholderTextColor={colors.textSecondary}
              textAlignVertical="top"
            />
          </View>
        </Card>

        {submitError && (
          <View style={styles.errorBanner}>
            <Text style={styles.errorText}>{submitError}</Text>
          </View>
        )}

        {isOffline && (
          <View style={styles.offlineBanner}>
            <WifiOff size={16} color={colors.warning} />
            <Text style={styles.offlineText}>
              {t('pod.offlineMessage')}
            </Text>
          </View>
        )}

        <View style={styles.submitContainer}>
          <Button
            title={isSubmitting ? t('pod.submitting') : t('pod.submitDelivery')}
            onPress={handleSubmit}
            loading={isSubmitting}
            disabled={!isFormValid || isSubmitting}
            fullWidth
            size="lg"
          />
          {!isFormValid && (
            <Text style={styles.hint}>
              {!recipientName.trim()
                ? t('pod.enterRecipientName')
                : t('pod.captureSignature')}
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
  errorBanner: {
    backgroundColor: '#fee2e2',
    padding: spacing.md,
    borderRadius: 8,
    marginBottom: spacing.md,
  },
  errorText: {
    fontSize: 14,
    color: colors.error,
    textAlign: 'center',
  },
  offlineBanner: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: '#fef3c7',
    padding: spacing.md,
    borderRadius: 8,
    marginBottom: spacing.md,
    gap: spacing.sm,
  },
  offlineText: {
    fontSize: 14,
    color: '#92400e',
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
export { PODFormProps };
