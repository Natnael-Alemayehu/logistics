import { View, Text, StyleSheet, Pressable, Modal, TextInput } from 'react-native';
import { useState } from 'react';
import { colors, spacing } from '../../utils/theme';

interface DelayReasonPickerProps {
  visible: boolean;
  onSelect: (reason: string, note?: string) => void;
  onClose: () => void;
}

const DELAY_REASONS = [
  { id: 'road_conditions', label: 'Road Conditions', icon: '🛣️' },
  { id: 'weather', label: 'Weather', icon: '🌤️' },
  { id: 'security_checkpoint', label: 'Security Checkpoint', icon: '🛡️' },
  { id: 'mechanical_issue', label: 'Mechanical Issue', icon: '🔧' },
  { id: 'traffic', label: 'Traffic', icon: '🚗' },
  { id: 'other', label: 'Other', icon: '📝' },
];

export function DelayReasonPicker({ visible, onSelect, onClose }: DelayReasonPickerProps) {
  const [selectedReason, setSelectedReason] = useState<string | null>(null);
  const [otherNote, setOtherNote] = useState('');
  const [showOtherInput, setShowOtherInput] = useState(false);

  const handleReasonSelect = (reasonId: string) => {
    setSelectedReason(reasonId);
    if (reasonId === 'other') {
      setShowOtherInput(true);
    } else {
      setShowOtherInput(false);
      setOtherNote('');
    }
  };

  const handleConfirm = () => {
    if (selectedReason) {
      onSelect(selectedReason, selectedReason === 'other' ? otherNote : undefined);
      handleReset();
    }
  };

  const handleReset = () => {
    setSelectedReason(null);
    setOtherNote('');
    setShowOtherInput(false);
  };

  const handleClose = () => {
    handleReset();
    onClose();
  };

  return (
    <Modal
      visible={visible}
      transparent
      animationType="slide"
      onRequestClose={handleClose}
    >
      <Pressable style={styles.overlay} onPress={handleClose}>
        <Pressable style={styles.container} onPress={(e) => e.stopPropagation()}>
          <View style={styles.handle} />
          
          <Text style={styles.title}>Select Delay Reason</Text>
          
          <View style={styles.reasonsList}>
            {DELAY_REASONS.map((reason) => (
              <Pressable
                key={reason.id}
                style={[
                  styles.reasonItem,
                  selectedReason === reason.id && styles.reasonItemSelected,
                ]}
                onPress={() => handleReasonSelect(reason.id)}
              >
                <Text style={styles.reasonIcon}>{reason.icon}</Text>
                <Text
                  style={[
                    styles.reasonLabel,
                    selectedReason === reason.id && styles.reasonLabelSelected,
                  ]}
                >
                  {reason.label}
                </Text>
                {selectedReason === reason.id && (
                  <Text style={styles.checkmark}>✓</Text>
                )}
              </Pressable>
            ))}
          </View>

          {showOtherInput && (
            <View style={styles.otherInputContainer}>
              <Text style={styles.otherLabel}>Please describe:</Text>
              <TextInput
                style={styles.otherInput}
                value={otherNote}
                onChangeText={setOtherNote}
                placeholder="Enter details..."
                placeholderTextColor={colors.textSecondary}
                multiline
                numberOfLines={3}
              />
            </View>
          )}

          <View style={styles.buttons}>
            <Pressable style={styles.cancelButton} onPress={handleClose}>
              <Text style={styles.cancelText}>Cancel</Text>
            </Pressable>
            <Pressable
              style={[
                styles.confirmButton,
                !selectedReason && styles.confirmButtonDisabled,
              ]}
              onPress={handleConfirm}
              disabled={!selectedReason}
            >
              <Text style={styles.confirmText}>Confirm</Text>
            </Pressable>
          </View>
        </Pressable>
      </Pressable>
    </Modal>
  );
}

const styles = StyleSheet.create({
  overlay: {
    flex: 1,
    backgroundColor: 'rgba(0, 0, 0, 0.5)',
    justifyContent: 'flex-end',
  },
  container: {
    backgroundColor: colors.surface,
    borderTopLeftRadius: 20,
    borderTopRightRadius: 20,
    padding: spacing.lg,
    paddingBottom: spacing.xxl,
  },
  handle: {
    width: 40,
    height: 4,
    backgroundColor: colors.border,
    borderRadius: 2,
    alignSelf: 'center',
    marginBottom: spacing.lg,
  },
  title: {
    fontSize: 18,
    fontWeight: '600',
    color: colors.text,
    marginBottom: spacing.lg,
    textAlign: 'center',
  },
  reasonsList: {
    gap: spacing.sm,
  },
  reasonItem: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: spacing.md,
    borderRadius: 8,
    backgroundColor: colors.background,
    borderWidth: 1,
    borderColor: colors.border,
  },
  reasonItemSelected: {
    backgroundColor: '#dbeafe',
    borderColor: colors.primary,
  },
  reasonIcon: {
    fontSize: 20,
    marginRight: spacing.md,
  },
  reasonLabel: {
    flex: 1,
    fontSize: 15,
    color: colors.text,
  },
  reasonLabelSelected: {
    color: colors.primary,
    fontWeight: '500',
  },
  checkmark: {
    fontSize: 16,
    color: colors.primary,
    fontWeight: 'bold',
  },
  otherInputContainer: {
    marginTop: spacing.md,
  },
  otherLabel: {
    fontSize: 14,
    color: colors.textSecondary,
    marginBottom: spacing.sm,
  },
  otherInput: {
    backgroundColor: colors.background,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: spacing.md,
    fontSize: 15,
    color: colors.text,
    minHeight: 80,
    textAlignVertical: 'top',
  },
  buttons: {
    flexDirection: 'row',
    gap: spacing.md,
    marginTop: spacing.xl,
  },
  cancelButton: {
    flex: 1,
    paddingVertical: spacing.md,
    borderRadius: 8,
    backgroundColor: colors.background,
    alignItems: 'center',
  },
  cancelText: {
    fontSize: 15,
    color: colors.textSecondary,
    fontWeight: '500',
  },
  confirmButton: {
    flex: 1,
    paddingVertical: spacing.md,
    borderRadius: 8,
    backgroundColor: colors.primary,
    alignItems: 'center',
  },
  confirmButtonDisabled: {
    opacity: 0.5,
  },
  confirmText: {
    fontSize: 15,
    color: '#ffffff',
    fontWeight: '600',
  },
});
