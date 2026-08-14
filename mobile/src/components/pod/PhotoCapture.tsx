import React, { useState, useCallback, useRef } from 'react';
import {
  View,
  StyleSheet,
  TouchableOpacity,
  Text,
  Image,
  Alert,
  Dimensions,
  ActivityIndicator,
  Modal,
} from 'react-native';
import { CameraView, CameraType, useCameraPermissions } from 'expo-camera';
import * as Location from 'expo-location';
import { Paths, Directory, File } from 'expo-file-system';
import * as ImageManipulator from 'expo-image-manipulator';
import { X, Camera, RotateCcw, Check, RefreshCw } from 'lucide-react-native';
import { generateUUID } from '../../utils/helpers';
import { colors, spacing } from '../../utils/theme';
import { PODPhoto } from '../../types/pod';
import { useTranslation } from 'react-i18next';

const { width, height } = Dimensions.get('window');
const THUMBNAIL_SIZE = (width - spacing.xl * 2 - spacing.md * 2) / 3;

interface PhotoCaptureProps {
  photos: PODPhoto[];
  onAddPhoto: (photo: PODPhoto) => void;
  onRemovePhoto: (index: number) => void;
  maxPhotos?: number;
}

const PhotoCapture: React.FC<PhotoCaptureProps> = ({
  photos,
  onAddPhoto,
  onRemovePhoto,
  maxPhotos = 3,
}) => {
  const { t } = useTranslation();
  const [cameraVisible, setCameraVisible] = useState(false);
  const [previewPhoto, setPreviewPhoto] = useState<Partial<PODPhoto> | null>(null);
  const [capturing, setCapturing] = useState(false);
  const [processing, setProcessing] = useState(false);
  const canAddPhoto = photos.length < maxPhotos;
  const remainingSlots = maxPhotos - photos.length;

  const handleOpenCamera = useCallback(() => {
    if (!canAddPhoto) {
      Alert.alert(t('pod.limitReached'), t('pod.maxPhotosAllowed', { max: maxPhotos }));
      return;
    }
    setCameraVisible(true);
    setPreviewPhoto(null);
  }, [canAddPhoto, maxPhotos, t]);

  const handleCloseCamera = useCallback(() => {
    setCameraVisible(false);
    setPreviewPhoto(null);
  }, []);

  const handleRetake = useCallback(() => {
    setPreviewPhoto(null);
  }, []);

  const handleConfirmPhoto = useCallback(() => {
    if (previewPhoto) {
      onAddPhoto(previewPhoto as PODPhoto);
      setPreviewPhoto(null);
      setCameraVisible(false);
    }
  }, [previewPhoto, onAddPhoto]);

  return (
    <View style={styles.container}>
      <View style={styles.photoGrid}>
        {photos.map((photo, index) => (
          <View key={photo.id || index} style={styles.photoContainer}>
            <Image
              source={{ uri: photo.thumbnailUri || photo.localUri }}
              style={styles.thumbnail}
              resizeMode="cover"
            />
            <TouchableOpacity
              style={styles.removeButton}
              onPress={() => onRemovePhoto(index)}
              hitSlop={{ top: 10, bottom: 10, left: 10, right: 10 }}
            >
              <X size={14} color={colors.surface} />
            </TouchableOpacity>
          </View>
        ))}
        
        {canAddPhoto && (
          <TouchableOpacity style={styles.addButton} onPress={handleOpenCamera}>
            <View style={styles.addButtonContent}>
              <Camera size={24} color={colors.textSecondary} />
              <Text style={styles.addButtonText}>{t('pod.addPhoto')}</Text>
            </View>
          </TouchableOpacity>
        )}
      </View>
      
      <Text style={styles.hint}>
        {photos.length === 0 
          ? t('pod.addUpToPhotos', { max: maxPhotos })
          : t('pod.photosRemaining', { count: remainingSlots })
        }
      </Text>

      <Modal
        visible={cameraVisible}
        animationType="slide"
        presentationStyle="fullScreen"
        onRequestClose={handleCloseCamera}
      >
        <CameraModal
          onClose={handleCloseCamera}
          onCapture={setPreviewPhoto}
          previewPhoto={previewPhoto}
          onRetake={handleRetake}
          onConfirm={handleConfirmPhoto}
          capturing={capturing}
          setCapturing={setCapturing}
          processing={processing}
          setProcessing={setProcessing}
        />
      </Modal>
    </View>
  );
};

interface CameraModalProps {
  onClose: () => void;
  onCapture: (photo: Partial<PODPhoto>) => void;
  previewPhoto: Partial<PODPhoto> | null;
  onRetake: () => void;
  onConfirm: () => void;
  capturing: boolean;
  setCapturing: (v: boolean) => void;
  processing: boolean;
  setProcessing: (v: boolean) => void;
}

const CameraModal: React.FC<CameraModalProps> = ({
  onClose,
  onCapture,
  previewPhoto,
  onRetake,
  onConfirm,
  capturing,
  setCapturing,
  processing,
  setProcessing,
}) => {
  const { t } = useTranslation();
  const [permission, requestPermission] = useCameraPermissions();
  const [locationPermission, setLocationPermission] = useState<boolean | null>(null);
  const [facing, setFacing] = useState<CameraType>('back');
  const cameraRef = useRef<CameraView>(null);

  const requestLocationPermission = useCallback(async () => {
    const { status } = await Location.requestForegroundPermissionsAsync();
    setLocationPermission(status === 'granted');
    return status === 'granted';
  }, []);

  const getCurrentLocation = useCallback(async (): Promise<{ latitude: number; longitude: number } | null> => {
    try {
      if (locationPermission === null) {
        const granted = await requestLocationPermission();
        if (!granted) return null;
      }
      
      if (locationPermission === false) {
        return null;
      }

      const location = await Location.getCurrentPositionAsync({
        accuracy: Location.Accuracy.High,
      });

      return {
        latitude: location.coords.latitude,
        longitude: location.coords.longitude,
      };
    } catch {
      return null;
    }
  }, [locationPermission, requestLocationPermission]);

  const compressImage = useCallback(async (uri: string): Promise<string> => {
    const result = await ImageManipulator.manipulateAsync(
      uri,
      [{ resize: { width: 800, height: 600 } }],
      {
        compress: 0.7,
        format: ImageManipulator.SaveFormat.JPEG,
      }
    );
    return result.uri;
  }, []);

  const savePhotoToStorage = useCallback(async (uri: string): Promise<string> => {
    const photosDir = new Directory(Paths.document, 'pod_photos');
    
    if (!photosDir.exists) {
      photosDir.create();
    }

    const filename = `${generateUUID()}.jpg`;
    const newFile = new File(photosDir, filename);
    
    const sourceFile = new File(uri);
    sourceFile.copy(newFile);
    
    return newFile.uri;
  }, []);

  const getFileSize = useCallback(async (uri: string): Promise<number> => {
    try {
      const file = new File(uri);
      if (file.exists) {
        return file.size;
      }
      return 0;
    } catch {
      return 0;
    }
  }, []);

  const handleCapture = useCallback(async () => {
    if (!cameraRef.current || capturing || processing) return;

    setCapturing(true);
    
    try {
      const photo = await cameraRef.current.takePictureAsync({
        quality: 1,
        skipProcessing: false,
      });

      if (!photo) {
        Alert.alert(t('common.error'), t('pod.photoCaptureError'));
        setCapturing(false);
        return;
      }

      setProcessing(true);

      const location = await getCurrentLocation();
      const timestamp = new Date();
      
      const compressedUri = await compressImage(photo.uri);
      
      const savedUri = await savePhotoToStorage(compressedUri);
      const fileSize = await getFileSize(savedUri);

      const photosDir = new Directory(Paths.document, 'pod_photos');
      const thumbFilename = `thumb_${generateUUID()}.jpg`;
      const thumbnailFile = new File(photosDir, thumbFilename);

      const thumbnailResult = await ImageManipulator.manipulateAsync(
        photo.uri,
        [{ resize: { width: 200, height: 150 } }],
        { compress: 0.5, format: ImageManipulator.SaveFormat.JPEG }
      );

      const thumbSource = new File(thumbnailResult.uri);
      thumbSource.copy(thumbnailFile);

      const podPhoto: Partial<PODPhoto> = {
        id: generateUUID(),
        localUri: savedUri,
        thumbnailUri: thumbnailFile.uri,
        latitude: location?.latitude,
        longitude: location?.longitude,
        capturedAt: timestamp.toISOString(),
        fileSize,
        width: 800,
        height: 600,
        uploaded: false,
      };

      onCapture(podPhoto);
      
      try {
        const tempFile = new File(photo.uri);
        if (tempFile.exists) {
          tempFile.delete();
        }
      } catch {}
      
    } catch (error) {
      console.error('Capture error:', error);
      Alert.alert(t('common.error'), t('pod.photoProcessError'));
    } finally {
      setCapturing(false);
      setProcessing(false);
    }
  }, [capturing, processing, getCurrentLocation, compressImage, savePhotoToStorage, getFileSize, onCapture, setProcessing, t]);

  const toggleCameraFacing = useCallback(() => {
    setFacing(current => current === 'back' ? 'front' : 'back');
  }, []);

  if (!permission) {
    return (
      <View style={styles.loadingContainer}>
        <ActivityIndicator size="large" color={colors.primary} />
      </View>
    );
  }

  if (!permission.granted) {
    return (
      <View style={styles.permissionContainer}>
        <Camera size={48} color={colors.textSecondary} />
        <Text style={styles.permissionTitle}>{t('pod.cameraAccessRequired')}</Text>
        <Text style={styles.permissionText}>
          {t('pod.cameraAccessMessage')}
        </Text>
        <TouchableOpacity style={styles.permissionButton} onPress={requestPermission}>
          <Text style={styles.permissionButtonText}>{t('pod.grantPermission')}</Text>
        </TouchableOpacity>
        <TouchableOpacity style={styles.cancelButton} onPress={onClose}>
          <Text style={styles.cancelButtonText}>{t('common.cancel')}</Text>
        </TouchableOpacity>
      </View>
    );
  }

  if (processing) {
    return (
      <View style={styles.loadingContainer}>
        <ActivityIndicator size="large" color={colors.primary} />
        <Text style={styles.processingText}>{t('pod.processingPhoto')}</Text>
      </View>
    );
  }

  if (previewPhoto?.localUri) {
    return (
      <View style={styles.previewContainer}>
        <Image
          source={{ uri: previewPhoto.localUri }}
          style={styles.previewImage}
          resizeMode="contain"
        />
        
        {previewPhoto.latitude && previewPhoto.longitude && (
          <View style={styles.locationBadge}>
            <Text style={styles.locationText}>
              {t('pod.locationTagged')}
            </Text>
          </View>
        )}
        
        <View style={styles.previewActions}>
          <TouchableOpacity style={styles.retakeButton} onPress={onRetake}>
            <RefreshCw size={24} color={colors.text} />
            <Text style={styles.retakeButtonText}>{t('pod.retake')}</Text>
          </TouchableOpacity>
          
          <TouchableOpacity style={styles.confirmButton} onPress={onConfirm}>
            <Check size={24} color={colors.surface} />
            <Text style={styles.confirmButtonText}>{t('pod.usePhoto')}</Text>
          </TouchableOpacity>
        </View>
      </View>
    );
  }

  return (
    <View style={styles.cameraContainer}>
      <CameraView
        ref={cameraRef}
        style={styles.camera}
        facing={facing}
        mode="picture"
      />
      
      <View style={styles.cameraOverlay}>
        <TouchableOpacity style={styles.closeButton} onPress={onClose}>
          <X size={28} color={colors.surface} />
        </TouchableOpacity>
        
        <TouchableOpacity style={styles.flipButton} onPress={toggleCameraFacing}>
          <RotateCcw size={24} color={colors.surface} />
        </TouchableOpacity>
      </View>
      
      <View style={styles.captureContainer}>
        <TouchableOpacity
          style={[styles.captureButton, capturing && styles.captureButtonDisabled]}
          onPress={handleCapture}
          disabled={capturing}
        >
          {capturing ? (
            <ActivityIndicator size="small" color={colors.surface} />
          ) : (
            <View style={styles.captureButtonInner} />
          )}
        </TouchableOpacity>
      </View>
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    width: '100%',
  },
  photoGrid: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.md,
  },
  photoContainer: {
    width: THUMBNAIL_SIZE,
    height: THUMBNAIL_SIZE,
    position: 'relative',
  },
  thumbnail: {
    width: '100%',
    height: '100%',
    borderRadius: 8,
    backgroundColor: colors.background,
  },
  removeButton: {
    position: 'absolute',
    top: -spacing.xs,
    right: -spacing.xs,
    width: 24,
    height: 24,
    borderRadius: 12,
    backgroundColor: colors.error,
    justifyContent: 'center',
    alignItems: 'center',
  },
  addButton: {
    width: THUMBNAIL_SIZE,
    height: THUMBNAIL_SIZE,
    borderWidth: 2,
    borderColor: colors.border,
    borderStyle: 'dashed',
    borderRadius: 8,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: colors.background,
  },
  addButtonContent: {
    alignItems: 'center',
    gap: spacing.xs,
  },
  addButtonText: {
    fontSize: 12,
    color: colors.textSecondary,
    fontWeight: '500',
  },
  hint: {
    fontSize: 12,
    color: colors.textSecondary,
    marginTop: spacing.sm,
    textAlign: 'center',
  },
  loadingContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: colors.background,
  },
  processingText: {
    marginTop: spacing.md,
    fontSize: 16,
    color: colors.textSecondary,
  },
  permissionContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    padding: spacing.xxl,
    backgroundColor: colors.background,
  },
  permissionTitle: {
    fontSize: 18,
    fontWeight: '600',
    color: colors.text,
    marginTop: spacing.lg,
    marginBottom: spacing.sm,
  },
  permissionText: {
    fontSize: 14,
    color: colors.textSecondary,
    textAlign: 'center',
    marginBottom: spacing.lg,
  },
  permissionButton: {
    backgroundColor: colors.primary,
    paddingVertical: spacing.md,
    paddingHorizontal: spacing.xl,
    borderRadius: 8,
  },
  permissionButtonText: {
    color: colors.surface,
    fontSize: 16,
    fontWeight: '600',
  },
  cancelButton: {
    marginTop: spacing.md,
    paddingVertical: spacing.md,
    paddingHorizontal: spacing.xl,
  },
  cancelButtonText: {
    color: colors.textSecondary,
    fontSize: 16,
  },
  cameraContainer: {
    flex: 1,
    backgroundColor: '#000',
  },
  camera: {
    flex: 1,
  },
  cameraOverlay: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    flexDirection: 'row',
    justifyContent: 'space-between',
    padding: spacing.lg,
    paddingTop: spacing.xxl * 2,
  },
  closeButton: {
    width: 44,
    height: 44,
    borderRadius: 22,
    backgroundColor: 'rgba(0,0,0,0.5)',
    justifyContent: 'center',
    alignItems: 'center',
  },
  flipButton: {
    width: 44,
    height: 44,
    borderRadius: 22,
    backgroundColor: 'rgba(0,0,0,0.5)',
    justifyContent: 'center',
    alignItems: 'center',
  },
  captureContainer: {
    position: 'absolute',
    bottom: spacing.xxl * 2,
    left: 0,
    right: 0,
    alignItems: 'center',
  },
  captureButton: {
    width: 72,
    height: 72,
    borderRadius: 36,
    backgroundColor: colors.surface,
    justifyContent: 'center',
    alignItems: 'center',
    borderWidth: 4,
    borderColor: 'rgba(255,255,255,0.5)',
  },
  captureButtonDisabled: {
    opacity: 0.7,
  },
  captureButtonInner: {
    width: 56,
    height: 56,
    borderRadius: 28,
    backgroundColor: colors.surface,
  },
  previewContainer: {
    flex: 1,
    backgroundColor: '#000',
  },
  previewImage: {
    flex: 1,
    width: '100%',
  },
  locationBadge: {
    position: 'absolute',
    top: spacing.xxl * 2,
    alignSelf: 'center',
    backgroundColor: 'rgba(0,0,0,0.7)',
    paddingVertical: spacing.xs,
    paddingHorizontal: spacing.md,
    borderRadius: 16,
  },
  locationText: {
    color: colors.surface,
    fontSize: 12,
  },
  previewActions: {
    position: 'absolute',
    bottom: 0,
    left: 0,
    right: 0,
    flexDirection: 'row',
    justifyContent: 'space-around',
    padding: spacing.xl,
    paddingBottom: spacing.xxl * 2,
    backgroundColor: 'rgba(0,0,0,0.8)',
  },
  retakeButton: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: spacing.md,
    paddingHorizontal: spacing.xl,
    backgroundColor: colors.surface,
    borderRadius: 8,
    gap: spacing.sm,
  },
  retakeButtonText: {
    color: colors.text,
    fontSize: 16,
    fontWeight: '600',
  },
  confirmButton: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: spacing.md,
    paddingHorizontal: spacing.xl,
    backgroundColor: colors.primary,
    borderRadius: 8,
    gap: spacing.sm,
  },
  confirmButtonText: {
    color: colors.surface,
    fontSize: 16,
    fontWeight: '600',
  },
});

export default PhotoCapture;
export { PhotoCaptureProps };
